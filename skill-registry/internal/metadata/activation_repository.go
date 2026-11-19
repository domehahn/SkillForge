package metadata

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func (r *Repository) GetActivation(ctx context.Context, namespace, kind, name string) (*Activation, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, namespace, kind, name, version, previous_version,
		desired_state, observed_state, generation, approved_by, approved_at, reason, last_error,
		created_by, created_at, updated_at FROM artifact_activations
		WHERE namespace = ? AND kind = ? AND name = ?`, namespace, kind, name)
	return scanActivation(row)
}

func (r *Repository) ListActivations(ctx context.Context, namespace string) ([]Activation, error) {
	query := `SELECT id, namespace, kind, name, version, previous_version, desired_state,
		observed_state, generation, approved_by, approved_at, reason, last_error, created_by,
		created_at, updated_at FROM artifact_activations`
	args := []any{}
	if namespace != "" {
		query += ` WHERE namespace = ?`
		args = append(args, namespace)
	}
	query += ` ORDER BY updated_at DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Activation
	for rows.Next() {
		a, err := scanActivation(rows)
		if err != nil {
			return nil, err
		}
		if a != nil {
			result = append(result, *a)
		}
	}
	return result, rows.Err()
}

func (r *Repository) UpsertActivation(ctx context.Context, activation *Activation, action, actor, fromState string) error {
	now := time.Now().UTC()
	if activation.CreatedAt.IsZero() {
		activation.CreatedAt = now
	}
	activation.UpdatedAt = now
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO artifact_activations
		(namespace, kind, name, version, previous_version, desired_state, observed_state,
		 generation, approved_by, approved_at, reason, last_error, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(namespace, kind, name) DO UPDATE SET
		 version=excluded.version, previous_version=excluded.previous_version,
		 desired_state=excluded.desired_state, observed_state=excluded.observed_state,
		 generation=excluded.generation, approved_by=excluded.approved_by,
		 approved_at=excluded.approved_at, reason=excluded.reason,
		 last_error=excluded.last_error, updated_at=excluded.updated_at`,
		activation.Namespace, activation.Kind, activation.Name, activation.Version,
		activation.PreviousVersion, activation.DesiredState, activation.ObservedState,
		activation.Generation, activation.ApprovedBy, activation.ApprovedAt,
		activation.Reason, activation.LastError, activation.CreatedBy,
		activation.CreatedAt, activation.UpdatedAt)
	if err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, `SELECT id FROM artifact_activations WHERE namespace=? AND kind=? AND name=?`,
		activation.Namespace, activation.Kind, activation.Name).Scan(&activation.ID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO activation_events
		(activation_id, generation, action, actor, from_state, to_state, reason, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, activation.ID, activation.Generation, action, actor,
		fromState, activation.DesiredState, activation.Reason, now)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) ListActivationEvents(ctx context.Context, activationID int64) ([]ActivationEvent, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, activation_id, generation, action, actor,
		from_state, to_state, reason, created_at FROM activation_events
		WHERE activation_id=? ORDER BY generation, id`, activationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ActivationEvent
	for rows.Next() {
		var e ActivationEvent
		if err := rows.Scan(&e.ID, &e.ActivationID, &e.Generation, &e.Action, &e.Actor,
			&e.FromState, &e.ToState, &e.Reason, &e.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

type rowScanner interface{ Scan(dest ...any) error }

func scanActivation(row rowScanner) (*Activation, error) {
	var a Activation
	var approvedAt sql.NullTime
	if err := row.Scan(&a.ID, &a.Namespace, &a.Kind, &a.Name, &a.Version,
		&a.PreviousVersion, &a.DesiredState, &a.ObservedState, &a.Generation,
		&a.ApprovedBy, &approvedAt, &a.Reason, &a.LastError, &a.CreatedBy,
		&a.CreatedAt, &a.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan activation: %w", err)
	}
	if approvedAt.Valid {
		a.ApprovedAt = &approvedAt.Time
	}
	return &a, nil
}
