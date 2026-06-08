package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (r *Repository) CreateArtifact(ctx context.Context, artifact *Artifact) error {
	tags, _ := json.Marshal(artifact.Tags)
	owners, _ := json.Marshal(artifact.Owners)
	now := time.Now()
	if artifact.Visibility == "" {
		artifact.Visibility = "public"
	}
	artifact.CreatedAt, artifact.UpdatedAt = now, now
	return r.db.QueryRowContext(ctx, `
		INSERT INTO artifacts (kind, namespace, name, description, latest_version, visibility, tags, owners, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(kind, namespace, name) DO UPDATE SET
			description = excluded.description, visibility = excluded.visibility,
			tags = excluded.tags, owners = excluded.owners, updated_at = excluded.updated_at
		RETURNING id
	`, artifact.Kind, artifact.Namespace, artifact.Name, artifact.Description, artifact.LatestVersion,
		artifact.Visibility, string(tags), string(owners), now, now).Scan(&artifact.ID)
}

func (r *Repository) GetArtifact(ctx context.Context, kind, namespace, name string) (*Artifact, error) {
	var a Artifact
	var tags, owners string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, kind, namespace, name, description, latest_version, visibility, tags, owners, created_at, updated_at
		FROM artifacts WHERE kind = ? AND namespace = ? AND name = ?
	`, kind, namespace, name).Scan(&a.ID, &a.Kind, &a.Namespace, &a.Name, &a.Description,
		&a.LatestVersion, &a.Visibility, &tags, &owners, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(tags), &a.Tags)
	_ = json.Unmarshal([]byte(owners), &a.Owners)
	a.Downloads, _ = r.ArtifactDownloadCount(ctx, a.ID, "")
	return &a, nil
}

func (r *Repository) ListArtifacts(ctx context.Context, kind, query, namespace string, limit, offset int) ([]Artifact, int, error) {
	var conditions []string
	var args []interface{}
	if kind != "" {
		conditions = append(conditions, "kind = ?")
		args = append(args, kind)
	}
	if namespace != "" {
		conditions = append(conditions, "namespace = ?")
		args = append(args, namespace)
	}
	if query != "" {
		conditions = append(conditions, "(name LIKE ? OR description LIKE ?)")
		args = append(args, "%"+query+"%", "%"+query+"%")
	}
	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM artifacts "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, kind, namespace, name, description, latest_version, visibility, tags, owners, created_at, updated_at
		FROM artifacts `+where+` ORDER BY updated_at DESC LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var artifacts []Artifact
	for rows.Next() {
		var a Artifact
		var tags, owners string
		if err := rows.Scan(&a.ID, &a.Kind, &a.Namespace, &a.Name, &a.Description, &a.LatestVersion,
			&a.Visibility, &tags, &owners, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal([]byte(tags), &a.Tags)
		_ = json.Unmarshal([]byte(owners), &a.Owners)
		a.Downloads, _ = r.ArtifactDownloadCount(ctx, a.ID, "")
		artifacts = append(artifacts, a)
	}
	return artifacts, total, rows.Err()
}

func (r *Repository) CreateArtifactVersion(ctx context.Context, v *ArtifactVersion) error {
	manifest, _ := json.Marshal(v.Manifest)
	lockfile, _ := json.Marshal(v.Lockfile)
	oci, _ := json.Marshal(v.OCIDescriptor)
	v.CreatedAt = time.Now()
	return r.db.QueryRowContext(ctx, `
		INSERT INTO artifact_versions (
			artifact_id, kind, namespace, name, version, digest_sha256, package_type, size_bytes,
			entrypoint, manifest, lockfile, oci_descriptor, signature_status, scan_status,
			validation_status, source, created_by, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id
	`, v.ArtifactID, v.Kind, v.Namespace, v.Name, v.Version, v.DigestSHA256, v.PackageType,
		v.SizeBytes, v.Entrypoint, string(manifest), string(lockfile), string(oci),
		v.SignatureStatus, v.ScanStatus, v.ValidationStatus, v.Source, v.CreatedBy, v.CreatedAt).Scan(&v.ID)
}

func (r *Repository) GetArtifactVersion(ctx context.Context, kind, namespace, name, version string) (*ArtifactVersion, error) {
	var v ArtifactVersion
	var manifest, lockfile, oci string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, artifact_id, kind, namespace, name, version, digest_sha256, package_type, size_bytes,
			entrypoint, manifest, lockfile, oci_descriptor, signature_status, scan_status,
			validation_status, source, created_by, created_at
		FROM artifact_versions WHERE kind = ? AND namespace = ? AND name = ? AND version = ?
	`, kind, namespace, name, version).Scan(&v.ID, &v.ArtifactID, &v.Kind, &v.Namespace, &v.Name,
		&v.Version, &v.DigestSHA256, &v.PackageType, &v.SizeBytes, &v.Entrypoint, &manifest,
		&lockfile, &oci, &v.SignatureStatus, &v.ScanStatus, &v.ValidationStatus, &v.Source,
		&v.CreatedBy, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(manifest), &v.Manifest)
	if lockfile != "" && lockfile != "null" {
		_ = json.Unmarshal([]byte(lockfile), &v.Lockfile)
	}
	_ = json.Unmarshal([]byte(oci), &v.OCIDescriptor)
	v.Downloads, _ = r.ArtifactDownloadCount(ctx, v.ArtifactID, v.Version)
	return &v, nil
}

func (r *Repository) ListArtifactVersions(ctx context.Context, artifactID int64) ([]ArtifactVersion, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT version FROM artifact_versions WHERE artifact_id = ? ORDER BY created_at DESC`, artifactID)
	if err != nil {
		return nil, err
	}
	var versionNames []string
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			rows.Close()
			return nil, err
		}
		versionNames = append(versionNames, version)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	var versions []ArtifactVersion
	for _, version := range versionNames {
		var kind, namespace, name string
		if err := r.db.QueryRowContext(ctx, `SELECT kind, namespace, name FROM artifacts WHERE id = ?`, artifactID).Scan(&kind, &namespace, &name); err != nil {
			return nil, err
		}
		v, err := r.GetArtifactVersion(ctx, kind, namespace, name, version)
		if err != nil {
			return nil, err
		}
		versions = append(versions, *v)
	}
	return versions, nil
}

func (r *Repository) SetArtifactDistTag(ctx context.Context, artifactID int64, tag, version, actor string) error {
	var exists int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM artifact_versions WHERE artifact_id = ? AND version = ?`, artifactID, version).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return fmt.Errorf("version not found")
	}
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO artifact_dist_tags (artifact_id, tag, version, updated_at, updated_by)
		VALUES (?, ?, ?, ?, ?) ON CONFLICT(artifact_id, tag) DO UPDATE SET
		version = excluded.version, updated_at = excluded.updated_at, updated_by = excluded.updated_by
	`, artifactID, tag, version, now, actor)
	if err == nil && tag == "latest" {
		_, err = r.db.ExecContext(ctx, `UPDATE artifacts SET latest_version = ?, updated_at = ? WHERE id = ?`, version, now, artifactID)
	}
	return err
}

func (r *Repository) ListArtifactDistTags(ctx context.Context, artifactID int64) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT tag, version FROM artifact_dist_tags WHERE artifact_id = ? ORDER BY tag`, artifactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := map[string]string{}
	for rows.Next() {
		var tag, version string
		if err := rows.Scan(&tag, &version); err != nil {
			return nil, err
		}
		tags[tag] = version
	}
	return tags, rows.Err()
}

func (r *Repository) IncrementArtifactDownload(ctx context.Context, artifactID int64, version string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO artifact_download_counts (artifact_id, version, count, updated_at) VALUES (?, ?, 1, ?)
		ON CONFLICT(artifact_id, version) DO UPDATE SET count = count + 1, updated_at = excluded.updated_at
	`, artifactID, version, time.Now())
	return err
}

func (r *Repository) ArtifactDownloadCount(ctx context.Context, artifactID int64, version string) (int64, error) {
	query := `SELECT COALESCE(SUM(count), 0) FROM artifact_download_counts WHERE artifact_id = ?`
	args := []interface{}{artifactID}
	if version != "" {
		query += ` AND version = ?`
		args = append(args, version)
	}
	var count int64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func (r *Repository) RecordPromotion(ctx context.Context, p *Promotion) error {
	p.CreatedAt = time.Now()
	result, err := r.db.ExecContext(ctx, `INSERT INTO promotions (kind, namespace, name, version, channel, actor, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.Kind, p.Namespace, p.Name, p.Version, p.Channel, p.Actor, p.CreatedAt)
	if err != nil {
		return err
	}
	p.ID, _ = result.LastInsertId()
	return nil
}

func (r *Repository) ListPromotions(ctx context.Context, kind, namespace, name string) ([]Promotion, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, kind, namespace, name, version, channel, actor, created_at FROM promotions WHERE kind = ? AND namespace = ? AND name = ? ORDER BY created_at DESC`, kind, namespace, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var promotions []Promotion
	for rows.Next() {
		var promotion Promotion
		if err := rows.Scan(&promotion.ID, &promotion.Kind, &promotion.Namespace, &promotion.Name, &promotion.Version, &promotion.Channel, &promotion.Actor, &promotion.CreatedAt); err != nil {
			return nil, err
		}
		promotions = append(promotions, promotion)
	}
	return promotions, rows.Err()
}

func (r *Repository) SetArtifactAttestation(ctx context.Context, id int64, signatureStatus, scanStatus string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE artifact_versions SET signature_status = COALESCE(NULLIF(?, ''), signature_status),
			scan_status = COALESCE(NULLIF(?, ''), scan_status) WHERE id = ?
	`, signatureStatus, scanStatus, id)
	return err
}

func (r *Repository) CreateAttestation(ctx context.Context, attestation *Attestation) error {
	predicate, _ := json.Marshal(attestation.Predicate)
	attestation.CreatedAt = time.Now()
	result, err := r.db.ExecContext(ctx, `INSERT INTO attestations (artifact_version_id, type, digest, predicate, created_by, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		attestation.ArtifactVersionID, attestation.Type, attestation.Digest, string(predicate), attestation.CreatedBy, attestation.CreatedAt)
	if err != nil {
		return err
	}
	attestation.ID, _ = result.LastInsertId()
	return nil
}

func (r *Repository) ListAttestations(ctx context.Context, artifactVersionID int64) ([]Attestation, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, artifact_version_id, type, digest, predicate, created_by, created_at FROM attestations WHERE artifact_version_id = ? ORDER BY created_at DESC`, artifactVersionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var attestations []Attestation
	for rows.Next() {
		var attestation Attestation
		var predicate string
		if err := rows.Scan(&attestation.ID, &attestation.ArtifactVersionID, &attestation.Type, &attestation.Digest, &predicate, &attestation.CreatedBy, &attestation.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(predicate), &attestation.Predicate)
		attestations = append(attestations, attestation)
	}
	return attestations, rows.Err()
}

func (r *Repository) UpsertNamespaceMember(ctx context.Context, member *NamespaceMember) error {
	member.CreatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO namespace_members (namespace, username, role, created_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(namespace, username) DO UPDATE SET role = excluded.role
	`, member.Namespace, member.Username, member.Role, member.CreatedAt)
	return err
}

func (r *Repository) ListNamespaceMembers(ctx context.Context, namespace string) ([]NamespaceMember, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT namespace, username, role, created_at FROM namespace_members WHERE namespace = ? ORDER BY username`, namespace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var members []NamespaceMember
	for rows.Next() {
		var member NamespaceMember
		if err := rows.Scan(&member.Namespace, &member.Username, &member.Role, &member.CreatedAt); err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (r *Repository) NamespaceRole(ctx context.Context, namespace, username string) (string, error) {
	var role string
	err := r.db.QueryRowContext(ctx, `SELECT role FROM namespace_members WHERE namespace = ? AND username = ?`, namespace, username).Scan(&role)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return role, err
}

func (r *Repository) CreateWebhook(ctx context.Context, webhook *Webhook) error {
	events, _ := json.Marshal(webhook.Events)
	webhook.CreatedAt, webhook.Active = time.Now(), true
	result, err := r.db.ExecContext(ctx, `INSERT INTO webhooks (namespace, url, events, active, created_at) VALUES (?, ?, ?, 1, ?)`,
		webhook.Namespace, webhook.URL, string(events), webhook.CreatedAt)
	if err != nil {
		return err
	}
	webhook.ID, _ = result.LastInsertId()
	return nil
}

func (r *Repository) ListWebhooks(ctx context.Context, namespace string) ([]Webhook, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, namespace, url, events, active, created_at FROM webhooks WHERE namespace = ? ORDER BY id`, namespace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hooks []Webhook
	for rows.Next() {
		var hook Webhook
		var events string
		var active int
		if err := rows.Scan(&hook.ID, &hook.Namespace, &hook.URL, &events, &active, &hook.CreatedAt); err != nil {
			return nil, err
		}
		hook.Active = active != 0
		_ = json.Unmarshal([]byte(events), &hook.Events)
		hooks = append(hooks, hook)
	}
	return hooks, rows.Err()
}
