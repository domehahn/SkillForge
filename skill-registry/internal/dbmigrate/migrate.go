package dbmigrate

import (
	"database/sql"
	"fmt"
)

// Run applies migration `id` exactly once. If id already exists in
// schema_migrations the function returns nil immediately. Otherwise fn is
// called inside a transaction; on success the id is recorded.
func Run(db *sql.DB, id string, fn func(tx *sql.Tx) error) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		id         TEXT PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		return fmt.Errorf("schema_migrations: %w", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE id = ?`, id).Scan(&count); err != nil {
		return fmt.Errorf("schema_migrations query: %w", err)
	}
	if count > 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", id, err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("migration %s: %w", id, err)
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations (id) VALUES (?)`, id); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("recording migration %s: %w", id, err)
	}
	return tx.Commit()
}
