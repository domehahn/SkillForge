package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/skillforge/skill-registry/internal/dbmigrate"
)

type Entry struct {
	ID        int64           `json:"id"`
	Actor     string          `json:"actor"`
	Action    string          `json:"action"`
	Target    string          `json:"target"`
	Details   json.RawMessage `json:"details,omitempty"`
	IPAddress string          `json:"ip_address,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) (*Repository, error) {
	r := &Repository{db: db}
	if err := r.migrate(); err != nil {
		return nil, fmt.Errorf("audit migration: %w", err)
	}
	return r, nil
}

func (r *Repository) migrate() error {
	if err := dbmigrate.Run(r.db, "audit_001_initial", func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			CREATE TABLE IF NOT EXISTS audit_log (
				id         INTEGER PRIMARY KEY AUTOINCREMENT,
				actor      TEXT NOT NULL,
				action     TEXT NOT NULL,
				target     TEXT NOT NULL DEFAULT '',
				details    TEXT,
				ip_address TEXT NOT NULL DEFAULT '',
				created_at DATETIME NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_audit_actor   ON audit_log(actor);
			CREATE INDEX IF NOT EXISTS idx_audit_action  ON audit_log(action);
			CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_log(created_at);
		`)
		return err
	}); err != nil {
		return err
	}
	return dbmigrate.Run(r.db, "audit_002_compat_columns", func(tx *sql.Tx) error {
		for _, stmt := range []string{
			`ALTER TABLE audit_log ADD COLUMN target TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE audit_log ADD COLUMN details TEXT`,
			`ALTER TABLE audit_log ADD COLUMN ip_address TEXT NOT NULL DEFAULT ''`,
		} {
			if _, err := tx.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column") && !strings.Contains(err.Error(), "already exists") {
				return err
			}
		}
		_, err := tx.Exec(`
			CREATE INDEX IF NOT EXISTS idx_audit_actor   ON audit_log(actor);
			CREATE INDEX IF NOT EXISTS idx_audit_action  ON audit_log(action);
			CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_log(created_at);
		`)
		return err
	})
}

func (r *Repository) Log(ctx context.Context, e Entry) error {
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	var details *string
	if len(e.Details) > 0 {
		s := string(e.Details)
		details = &s
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO audit_log (actor, action, target, details, ip_address, created_at) VALUES (?,?,?,?,?,?)`,
		e.Actor, e.Action, e.Target, details, e.IPAddress, e.CreatedAt,
	)
	return err
}

func (r *Repository) List(ctx context.Context, actor, action string, limit int) ([]Entry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	q := `SELECT id, actor, action, target, details, ip_address, created_at FROM audit_log WHERE 1=1`
	args := []interface{}{}
	if actor != "" {
		q += " AND actor = ?"
		args = append(args, actor)
	}
	if action != "" {
		q += " AND action = ?"
		args = append(args, action)
	}
	q += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var details sql.NullString
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &e.Target, &details, &e.IPAddress, &e.CreatedAt); err != nil {
			return nil, err
		}
		if details.Valid {
			e.Details = json.RawMessage(details.String)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
