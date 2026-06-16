package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	DriverSQLite   = "sqlite"
	DriverPostgres = "postgres"
)

// Open creates a database handle for the configured backend.
func Open(driver, dsn string) (*sql.DB, error) {
	if driver == "" {
		driver = DriverSQLite
	}
	switch strings.ToLower(driver) {
	case DriverSQLite, "sqlite3":
		db, err := sql.Open("sqlite3", dsn)
		if err != nil {
			return nil, fmt.Errorf("open sqlite database: %w", err)
		}
		if err := configureSQLite(db); err != nil {
			db.Close()
			return nil, err
		}
		return db, nil
	case DriverPostgres, "postgresql":
		db, err := sql.Open("skillforge-postgres", dsn)
		if err != nil {
			return nil, fmt.Errorf("open postgres database: %w", err)
		}
		configurePostgres(db)
		return db, nil
	default:
		return nil, fmt.Errorf("unsupported database driver %q", driver)
	}
}

func configureSQLite(db *sql.DB) error {
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)
	for _, pragma := range []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA busy_timeout=5000`,
		`PRAGMA foreign_keys=ON`,
		`PRAGMA synchronous=NORMAL`,
	} {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("set sqlite pragma %q: %w", pragma, err)
		}
	}
	return nil
}

func configurePostgres(db *sql.DB) {
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(time.Hour)
}
