package dbmigrate_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/skillforge/skill-registry/internal/database"
	"github.com/skillforge/skill-registry/internal/dbmigrate"
)

func newTestDB(t *testing.T) *sql.DB {
	dbPath := filepath.Join(t.TempDir(), "test_migration.db")
	db, err := database.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	return db
}

// TestMigration_EmptyToLatest tests running migrations on an empty database.
func TestMigration_EmptyToLatest(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	if err := dbmigrate.Run(db, "v001_init", func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);`)
		return err
	}); err != nil {
		t.Fatalf("migration v001_init failed: %v", err)
	}

	if err := dbmigrate.Run(db, "v002_add_email", func(tx *sql.Tx) error {
		_, err := tx.Exec(`ALTER TABLE users ADD COLUMN email TEXT;`)
		return err
	}); err != nil {
		t.Fatalf("migration v002_add_email failed: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("failed to query schema_migrations: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 migrations recorded, got %d", count)
	}
}

// TestMigration_PreviousToLatest tests upgrading from a previous schema version to latest.
func TestMigration_PreviousToLatest(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	// Apply v1
	_ = dbmigrate.Run(db, "v001_init", func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY);`)
		return err
	})

	// Re-run v1 (should be no-op)
	_ = dbmigrate.Run(db, "v001_init", func(tx *sql.Tx) error {
		t.Error("v001_init should not be re-executed")
		return nil
	})

	// Apply v2
	if err := dbmigrate.Run(db, "v002_add_col", func(tx *sql.Tx) error {
		_, err := tx.Exec(`ALTER TABLE items ADD COLUMN title TEXT;`)
		return err
	}); err != nil {
		t.Fatalf("v002 failed: %v", err)
	}
}

// TestMigration_LatestToStartup tests starting up when all migrations are already applied.
func TestMigration_LatestToStartup(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	migrations := []string{"m1", "m2", "m3"}
	for _, m := range migrations {
		_ = dbmigrate.Run(db, m, func(tx *sql.Tx) error { return nil })
	}

	// Startup run
	for _, m := range migrations {
		executed := false
		err := dbmigrate.Run(db, m, func(tx *sql.Tx) error {
			executed = true
			return nil
		})
		if err != nil {
			t.Fatalf("startup check for %s failed: %v", m, err)
		}
		if executed {
			t.Errorf("migration %s was re-executed during startup", m)
		}
	}
}

// TestMigration_ConcurrentReplicaMigration tests concurrent migrations across multiple replicas.
func TestMigration_ConcurrentReplicaMigration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "concurrent_test.db")
	db, err := database.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open shared db: %v", err)
	}
	defer db.Close()

	const numReplicas = 5
	var wg sync.WaitGroup
	errs := make(chan error, numReplicas)

	for i := 0; i < numReplicas; i++ {
		wg.Add(1)
		go func(replicaID int) {
			defer wg.Done()
			err := dbmigrate.RunContext(context.Background(), db, "v1_shared_schema", func(tx *sql.Tx) error {
				_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS shared_resource (id INTEGER PRIMARY KEY, val TEXT);`)
				return err
			})
			if err != nil {
				errs <- fmt.Errorf("replica %d failed: %w", replicaID, err)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent migration error: %v", err)
	}
}

// TestMigration_InvalidMigrationFailsClosed tests that an invalid migration fails closed and rolls back.
func TestMigration_InvalidMigrationFailClosed(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	err := dbmigrate.Run(db, "v001_invalid", func(tx *sql.Tx) error {
		if _, err := tx.Exec(`CREATE TABLE temp_table (id INTEGER PRIMARY KEY);`); err != nil {
			return err
		}
		// Return deliberate error to trigger rollback
		return errors.New("simulated migration failure")
	})

	if err == nil {
		t.Fatal("expected invalid migration to fail closed")
	}

	// Verify temp_table was not created
	var tableCount int
	_ = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='temp_table'`).Scan(&tableCount)
	if tableCount != 0 {
		t.Errorf("expected temp_table to be rolled back, but found %d tables", tableCount)
	}

	// Verify migration was not recorded in schema_migrations
	var migCount int
	_ = db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE id='v001_invalid'`).Scan(&migCount)
	if migCount != 0 {
		t.Errorf("expected schema_migrations to have 0 entries for failed migration, got %d", migCount)
	}
}
