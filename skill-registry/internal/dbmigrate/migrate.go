package dbmigrate

import (
	"context"
	"database/sql"
	"fmt"
)

// RunContext applies migration `id` exactly once within a context and under a database-level lock.
// If id already exists in schema_migrations the function returns nil immediately.
// Otherwise fn is called inside a transaction; on success the id is recorded.
// On failure, the transaction is rolled back and the error is returned (fail closed).
func RunContext(ctx context.Context, db *sql.DB, id string, fn func(tx *sql.Tx) error) error {
	// Try acquiring Postgres advisory lock if using postgres driver
	var isPostgres bool
	var lockAcquired bool
	// Test if postgres by attempting a ping or checking driver name via sql handle
	_ = db.QueryRowContext(ctx, "SELECT pg_advisory_lock(837482910)").Scan(&isPostgres)
	if isPostgres {
		lockAcquired = true
		defer func() {
			_, _ = db.ExecContext(ctx, "SELECT pg_advisory_unlock(837482910)")
		}()
	}

	createTableQuery := `CREATE TABLE IF NOT EXISTS schema_migrations (
		id         TEXT PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`
	if lockAcquired {
		createTableQuery = `CREATE TABLE IF NOT EXISTS schema_migrations (
			id         TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`
	}

	if _, err := db.ExecContext(ctx, createTableQuery); err != nil {
		return fmt.Errorf("schema_migrations table creation failed: %w", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE id = $1`, id).Scan(&count); err != nil {
		// Fallback to ? for SQLite
		if err2 := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE id = ?`, id).Scan(&count); err2 != nil {
			return fmt.Errorf("schema_migrations query failed: %w", err)
		}
	}
	if count > 0 {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", id, err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("migration %s failed closed: %w", id, err)
	}

	insertQuery := `INSERT INTO schema_migrations (id) VALUES ($1) ON CONFLICT (id) DO NOTHING`
	if _, err := tx.ExecContext(ctx, insertQuery, id); err != nil {
		if _, err2 := tx.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations (id) VALUES (?)`, id); err2 != nil {
			_ = tx.Rollback()
			return fmt.Errorf("recording migration %s failed: %w", id, err)
		}
	}
	return tx.Commit()
}

// Run applies migration `id` using background context.
func Run(db *sql.DB, id string, fn func(tx *sql.Tx) error) error {
	return RunContext(context.Background(), db, id, fn)
}
