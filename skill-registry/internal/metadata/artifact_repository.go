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
		INSERT INTO artifacts (kind, namespace, name, description, readme, latest_version, visibility, tags, owners, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(kind, namespace, name) DO UPDATE SET
			description = excluded.description,
			readme = CASE WHEN excluded.readme != '' THEN excluded.readme ELSE readme END,
			visibility = excluded.visibility,
			tags = excluded.tags, owners = excluded.owners, updated_at = excluded.updated_at
		RETURNING id
	`, artifact.Kind, artifact.Namespace, artifact.Name, artifact.Description, artifact.Readme,
		artifact.LatestVersion, artifact.Visibility, string(tags), string(owners), now, now).Scan(&artifact.ID)
}

func (r *Repository) GetArtifact(ctx context.Context, kind, namespace, name string) (*Artifact, error) {
	var a Artifact
	var tags, owners string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, kind, namespace, name, description, readme, latest_version, visibility, tags, owners, created_at, updated_at
		FROM artifacts WHERE kind = ? AND namespace = ? AND name = ?
	`, kind, namespace, name).Scan(&a.ID, &a.Kind, &a.Namespace, &a.Name, &a.Description, &a.Readme,
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
	a.Stars = r.GetStarCount(ctx, a.ID)
	return &a, nil
}

func (r *Repository) ListArtifacts(ctx context.Context, kind, query, namespace, sort string, limit, offset int, verified bool) ([]Artifact, int, error) {
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
		conditions = append(conditions, "(name LIKE ? OR description LIKE ? OR tags LIKE ?)")
		args = append(args, "%"+query+"%", "%"+query+"%", "%"+query+"%")
	}
	if verified {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM namespace_profiles WHERE namespace_profiles.namespace = artifacts.namespace AND verified = 1)")
	}
	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}
	orderBy := "ORDER BY updated_at DESC"
	var orderArgs []interface{}
	switch sort {
	case "pulls":
		orderBy = "ORDER BY (SELECT COALESCE(SUM(count),0) FROM artifact_download_counts WHERE artifact_id = artifacts.id) DESC"
	case "stars":
		orderBy = "ORDER BY (SELECT COUNT(*) FROM artifact_stars WHERE artifact_stars.artifact_id = artifacts.id) DESC"
	case "name":
		orderBy = "ORDER BY name ASC"
	case "trending":
		orderBy = "ORDER BY (SELECT COALESCE(SUM(count),0) FROM artifact_download_trend WHERE artifact_id = artifacts.id AND day >= ?) DESC"
		orderArgs = append(orderArgs, time.Now().AddDate(0, 0, -7))
	}
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM artifacts "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	queryArgs := make([]interface{}, 0, len(args)+len(orderArgs)+2)
	queryArgs = append(queryArgs, args...)
	queryArgs = append(queryArgs, orderArgs...)
	queryArgs = append(queryArgs, limit, offset)
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, kind, namespace, name, description, latest_version, visibility, tags, owners, created_at, updated_at,
		       COALESCE((SELECT SUM(count) FROM artifact_download_counts WHERE artifact_id = artifacts.id), 0) AS downloads
		FROM artifacts `+where+" "+orderBy+` LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var artifacts []Artifact
	for rows.Next() {
		var a Artifact
		var tags, owners string
		if err := rows.Scan(&a.ID, &a.Kind, &a.Namespace, &a.Name, &a.Description, &a.LatestVersion,
			&a.Visibility, &tags, &owners, &a.CreatedAt, &a.UpdatedAt, &a.Downloads); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal([]byte(tags), &a.Tags)
		_ = json.Unmarshal([]byte(owners), &a.Owners)
		artifacts = append(artifacts, a)
	}
	return artifacts, total, rows.Err()
}

// KindFacets returns a map of kind→count for a given optional search query.
func (r *Repository) KindFacets(ctx context.Context, query string) (map[string]int, error) {
	var where string
	var args []interface{}
	if query != "" {
		where = "WHERE (name LIKE ? OR description LIKE ? OR tags LIKE ?)"
		args = append(args, "%"+query+"%", "%"+query+"%", "%"+query+"%")
	}
	rows, err := r.db.QueryContext(ctx, `SELECT kind, COUNT(*) FROM artifacts `+where+` GROUP BY kind`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	facets := make(map[string]int)
	for rows.Next() {
		var k string
		var c int
		if err := rows.Scan(&k, &c); err != nil {
			return nil, err
		}
		facets[k] = c
	}
	return facets, rows.Err()
}

// NamespaceStats returns artifact and pull counts for a namespace.
func (r *Repository) NamespaceStats(ctx context.Context, namespace string) (artifactCount int, totalPulls int64, err error) {
	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM artifacts WHERE namespace = ?`, namespace).Scan(&artifactCount)
	if err != nil {
		return
	}
	var pulls sql.NullInt64
	err = r.db.QueryRowContext(ctx, `
		SELECT SUM(adc.count)
		FROM artifact_download_counts adc
		JOIN artifacts a ON a.id = adc.artifact_id
		WHERE a.namespace = ?`, namespace).Scan(&pulls)
	if pulls.Valid {
		totalPulls = pulls.Int64
	}
	return
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
			validation_status, source, created_by, created_at,
			COALESCE(release_notes, '')
		FROM artifact_versions WHERE kind = ? AND namespace = ? AND name = ? AND version = ?
	`, kind, namespace, name, version).Scan(&v.ID, &v.ArtifactID, &v.Kind, &v.Namespace, &v.Name,
		&v.Version, &v.DigestSHA256, &v.PackageType, &v.SizeBytes, &v.Entrypoint, &manifest,
		&lockfile, &oci, &v.SignatureStatus, &v.ScanStatus, &v.ValidationStatus, &v.Source,
		&v.CreatedBy, &v.CreatedAt, &v.ReleaseNotes)
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
	now := time.Now()
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO artifact_download_counts (artifact_id, version, count, updated_at) VALUES (?, ?, 1, ?)
		ON CONFLICT(artifact_id, version) DO UPDATE SET count = count + 1, updated_at = excluded.updated_at
	`, artifactID, version, now); err != nil {
		return err
	}
	day := now.UTC().Format("2006-01-02")
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO artifact_download_trend (artifact_id, day, count) VALUES (?, ?, 1)
		ON CONFLICT(artifact_id, day) DO UPDATE SET count = count + 1
	`, artifactID, day)
	return err
}

func (r *Repository) GetArtifactDownloadTrend(ctx context.Context, artifactID int64, days int) ([]DayCount, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT day, count FROM artifact_download_trend
		WHERE artifact_id = ? AND day >= date('now', ?)
		ORDER BY day ASC
	`, artifactID, fmt.Sprintf("-%d days", days))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []DayCount
	for rows.Next() {
		var d DayCount
		if err := rows.Scan(&d.Day, &d.Count); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func (r *Repository) SetVersionReleaseNotes(ctx context.Context, kind, namespace, name, version, notes string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE artifact_versions SET release_notes = ?
		WHERE kind = ? AND namespace = ? AND name = ? AND version = ?
	`, notes, kind, namespace, name, version)
	return err
}

func (r *Repository) LogWebhookDelivery(ctx context.Context, d *WebhookDelivery) error {
	d.DeliveredAt = time.Now()
	return r.db.QueryRowContext(ctx, `
		INSERT INTO webhook_deliveries (webhook_id, namespace, event, status_code, duration_ms, success, delivered_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`, d.WebhookID, d.Namespace, d.Event, d.StatusCode, d.DurationMs, boolInt(d.Success), d.DeliveredAt).Scan(&d.ID)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (r *Repository) ListWebhookDeliveries(ctx context.Context, webhookID int64) ([]WebhookDelivery, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, webhook_id, namespace, event, status_code, duration_ms, success, delivered_at
		FROM webhook_deliveries WHERE webhook_id = ? ORDER BY delivered_at DESC LIMIT 50
	`, webhookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var deliveries []WebhookDelivery
	for rows.Next() {
		var d WebhookDelivery
		var success int
		if err := rows.Scan(&d.ID, &d.WebhookID, &d.Namespace, &d.Event, &d.StatusCode, &d.DurationMs, &success, &d.DeliveredAt); err != nil {
			return nil, err
		}
		d.Success = success == 1
		deliveries = append(deliveries, d)
	}
	return deliveries, rows.Err()
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
	return r.db.QueryRowContext(ctx, `INSERT INTO promotions (kind, namespace, name, version, channel, actor, created_at) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		p.Kind, p.Namespace, p.Name, p.Version, p.Channel, p.Actor, p.CreatedAt).Scan(&p.ID)
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
	return r.db.QueryRowContext(ctx, `INSERT INTO attestations (artifact_version_id, type, digest, predicate, created_by, created_at) VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
		attestation.ArtifactVersionID, attestation.Type, attestation.Digest, string(predicate), attestation.CreatedBy, attestation.CreatedAt).Scan(&attestation.ID)
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

func (r *Repository) RemoveNamespaceMember(ctx context.Context, namespace, username string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM namespace_members WHERE namespace = ? AND username = ?`, namespace, username)
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
	return r.db.QueryRowContext(ctx, `INSERT INTO webhooks (namespace, url, events, active, created_at) VALUES (?, ?, ?, 1, ?) RETURNING id`,
		webhook.Namespace, webhook.URL, string(events), webhook.CreatedAt).Scan(&webhook.ID)
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

func (r *Repository) DeleteWebhook(ctx context.Context, webhookID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM webhooks WHERE id = ?`, webhookID)
	return err
}

func (r *Repository) UpdateArtifactMeta(ctx context.Context, kind, namespace, name string, description, readme *string, tags []string, visibility *string) error {
	sets := []string{"updated_at = ?"}
	args := []interface{}{time.Now()}
	if description != nil {
		sets = append(sets, "description = ?")
		args = append(args, *description)
	}
	if readme != nil {
		sets = append(sets, "readme = ?")
		args = append(args, *readme)
	}
	if tags != nil {
		tagsJSON, _ := json.Marshal(tags)
		sets = append(sets, "tags = ?")
		args = append(args, string(tagsJSON))
	}
	if visibility != nil {
		sets = append(sets, "visibility = ?")
		args = append(args, *visibility)
	}
	if len(sets) == 1 {
		return nil
	}
	args = append(args, kind, namespace, name)
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(
		"UPDATE artifacts SET %s WHERE kind = ? AND namespace = ? AND name = ?",
		strings.Join(sets, ", "),
	), args...)
	return err
}

func (r *Repository) GetNamespaceBio(ctx context.Context, namespace string) string {
	var bio string
	_ = r.db.QueryRowContext(ctx, `SELECT bio FROM namespace_profiles WHERE namespace = ?`, namespace).Scan(&bio)
	return bio
}

func (r *Repository) SetNamespaceBio(ctx context.Context, namespace, bio string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO namespace_profiles (namespace, bio, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(namespace) DO UPDATE SET bio = excluded.bio, updated_at = excluded.updated_at
	`, namespace, bio, time.Now())
	return err
}

func (r *Repository) GetNamespaceVerified(ctx context.Context, namespace string) bool {
	var verified int
	_ = r.db.QueryRowContext(ctx, `SELECT COALESCE(verified, 0) FROM namespace_profiles WHERE namespace = ?`, namespace).Scan(&verified)
	return verified == 1
}

func (r *Repository) SetNamespaceVerified(ctx context.Context, namespace string, verified bool) error {
	v := 0
	if verified {
		v = 1
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO namespace_profiles (namespace, bio, verified, updated_at) VALUES (?, '', ?, ?)
		ON CONFLICT(namespace) DO UPDATE SET verified = excluded.verified, updated_at = excluded.updated_at
	`, namespace, v, time.Now())
	return err
}

func (r *Repository) GetDisplayName(ctx context.Context, namespace string) string {
	var name string
	_ = r.db.QueryRowContext(ctx, `SELECT COALESCE(display_name, '') FROM namespace_profiles WHERE namespace = ?`, namespace).Scan(&name)
	return name
}

func (r *Repository) SetDisplayName(ctx context.Context, namespace, displayName string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO namespace_profiles (namespace, bio, display_name, updated_at) VALUES (?, '', ?, ?)
		ON CONFLICT(namespace) DO UPDATE SET display_name = excluded.display_name, updated_at = excluded.updated_at
	`, namespace, displayName, time.Now())
	return err
}

type NamespaceInfo struct {
	Namespace     string `json:"namespace"`
	ArtifactCount int    `json:"artifact_count"`
	Verified      bool   `json:"verified"`
	Bio           string `json:"bio"`
	DisplayName   string `json:"display_name"`
}

type DependentArtifact struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func (r *Repository) GetArtifactUsedBy(ctx context.Context, namespace, name string) ([]DependentArtifact, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT av.kind, av.namespace, av.name
		FROM artifact_versions av
		WHERE json_extract(av.manifest, '$.spec.dependencies') IS NOT NULL
		  AND EXISTS (
			SELECT 1 FROM json_each(json_extract(av.manifest, '$.spec.dependencies')) dep
			WHERE json_extract(dep.value, '$.namespace') = ?
			  AND json_extract(dep.value, '$.name') = ?
		  )
		ORDER BY av.namespace, av.name
		LIMIT 20
	`, namespace, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []DependentArtifact
	for rows.Next() {
		var d DependentArtifact
		if err := rows.Scan(&d.Kind, &d.Namespace, &d.Name); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

type PublisherInfo struct {
	Namespace      string `json:"namespace"`
	ArtifactCount  int    `json:"artifact_count"`
	TotalDownloads int64  `json:"total_downloads"`
	Verified       bool   `json:"verified"`
	DisplayName    string `json:"display_name"`
}

func (r *Repository) ListTopPublishers(ctx context.Context, limit int) ([]PublisherInfo, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT a.namespace,
		       COUNT(DISTINCT a.id) as artifact_count,
		       COALESCE(SUM(adc.count), 0) as total_downloads,
		       COALESCE(np.verified, 0) as verified,
		       COALESCE(np.display_name, '') as display_name
		FROM artifacts a
		LEFT JOIN artifact_download_counts adc ON adc.artifact_id = a.id
		LEFT JOIN namespace_profiles np ON np.namespace = a.namespace
		WHERE a.visibility = 'public'
		GROUP BY a.namespace, np.verified, np.display_name
		ORDER BY total_downloads DESC, artifact_count DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []PublisherInfo
	for rows.Next() {
		var p PublisherInfo
		var verified int
		if err := rows.Scan(&p.Namespace, &p.ArtifactCount, &p.TotalDownloads, &verified, &p.DisplayName); err != nil {
			return nil, err
		}
		p.Verified = verified == 1
		result = append(result, p)
	}
	return result, rows.Err()
}

func (r *Repository) GetPinnedArtifacts(ctx context.Context, namespace string) []string {
	var raw string
	_ = r.db.QueryRowContext(ctx, `SELECT COALESCE(pinned, '[]') FROM namespace_profiles WHERE namespace = ?`, namespace).Scan(&raw)
	if raw == "" || raw == "null" {
		return nil
	}
	var pinned []string
	_ = json.Unmarshal([]byte(raw), &pinned)
	return pinned
}

func (r *Repository) SetPinnedArtifacts(ctx context.Context, namespace string, pinned []string) error {
	raw, _ := json.Marshal(pinned)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO namespace_profiles (namespace, bio, pinned, updated_at) VALUES (?, '', ?, ?)
		ON CONFLICT(namespace) DO UPDATE SET pinned = excluded.pinned, updated_at = excluded.updated_at
	`, namespace, string(raw), time.Now())
	return err
}

type Notification struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Link      string    `json:"link"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

func (r *Repository) AddNotification(ctx context.Context, n *Notification) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO notifications (username, type, message, link, created_at) VALUES (?, ?, ?, ?, ?)
	`, n.Username, n.Type, n.Message, n.Link, time.Now())
	return err
}

func (r *Repository) GetNotifications(ctx context.Context, username string) ([]Notification, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, username, type, message, link, read, created_at
		FROM notifications WHERE username = ?
		ORDER BY created_at DESC LIMIT 50
	`, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Notification
	for rows.Next() {
		var n Notification
		var read int
		if err := rows.Scan(&n.ID, &n.Username, &n.Type, &n.Message, &n.Link, &read, &n.CreatedAt); err != nil {
			return nil, err
		}
		n.Read = read == 1
		result = append(result, n)
	}
	return result, rows.Err()
}

func (r *Repository) MarkNotificationsRead(ctx context.Context, username string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE notifications SET read = 1 WHERE username = ?`, username)
	return err
}

func (r *Repository) DeleteNotification(ctx context.Context, id int64, username string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM notifications WHERE id = ? AND username = ?`, id, username)
	return err
}

func (r *Repository) ListAdminNamespaces(ctx context.Context) ([]NamespaceInfo, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT a.namespace, COUNT(*) as artifact_count,
		       COALESCE(np.verified, 0), COALESCE(np.bio, ''), COALESCE(np.display_name, '')
		FROM artifacts a
		LEFT JOIN namespace_profiles np ON np.namespace = a.namespace
		GROUP BY a.namespace
		ORDER BY artifact_count DESC, a.namespace
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []NamespaceInfo
	for rows.Next() {
		var ns NamespaceInfo
		var verified int
		if err := rows.Scan(&ns.Namespace, &ns.ArtifactCount, &verified, &ns.Bio, &ns.DisplayName); err != nil {
			return nil, err
		}
		ns.Verified = verified == 1
		result = append(result, ns)
	}
	return result, rows.Err()
}

// ── Stars ──────────────────────────────────────────────────────────────────

func (r *Repository) StarArtifact(ctx context.Context, artifactID int64, username string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO artifact_stars (artifact_id, username, created_at) VALUES (?, ?, ?)
	`, artifactID, username, time.Now())
	return err
}

func (r *Repository) UnstarArtifact(ctx context.Context, artifactID int64, username string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM artifact_stars WHERE artifact_id = ? AND username = ?`, artifactID, username)
	return err
}

func (r *Repository) GetStarCount(ctx context.Context, artifactID int64) int64 {
	var count int64
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM artifact_stars WHERE artifact_id = ?`, artifactID).Scan(&count)
	return count
}

func (r *Repository) IsStarred(ctx context.Context, artifactID int64, username string) bool {
	var count int
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM artifact_stars WHERE artifact_id = ? AND username = ?`, artifactID, username).Scan(&count)
	return count > 0
}

// ── Scan results ───────────────────────────────────────────────────────────

type ScanResult struct {
	ID                int64     `json:"id"`
	ArtifactVersionID int64     `json:"artifact_version_id"`
	Severity          string    `json:"severity"`
	CVEID             string    `json:"cve_id"`
	PackageName       string    `json:"package_name"`
	Description       string    `json:"description"`
	FixedVersion      string    `json:"fixed_version"`
	CreatedAt         time.Time `json:"created_at"`
}

func (r *Repository) AddScanResult(ctx context.Context, s *ScanResult) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO scan_results (artifact_version_id, severity, cve_id, package_name, description, fixed_version, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, s.ArtifactVersionID, s.Severity, s.CVEID, s.PackageName, s.Description, s.FixedVersion, time.Now())
	return err
}

func (r *Repository) ListScanResults(ctx context.Context, artifactVersionID int64) ([]ScanResult, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, artifact_version_id, severity, cve_id, package_name, description, fixed_version, created_at
		FROM scan_results WHERE artifact_version_id = ?
		ORDER BY CASE severity WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 WHEN 'low' THEN 3 ELSE 4 END, cve_id
	`, artifactVersionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []ScanResult
	for rows.Next() {
		var s ScanResult
		if err := rows.Scan(&s.ID, &s.ArtifactVersionID, &s.Severity, &s.CVEID, &s.PackageName, &s.Description, &s.FixedVersion, &s.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

func (r *Repository) ClearScanResults(ctx context.Context, artifactVersionID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM scan_results WHERE artifact_version_id = ?`, artifactVersionID)
	return err
}

// ── Comments ───────────────────────────────────────────────────────────────

type ArtifactComment struct {
	ID        int64     `json:"id"`
	Kind      string    `json:"kind"`
	Namespace string    `json:"namespace"`
	Name      string    `json:"name"`
	Username  string    `json:"username"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (r *Repository) AddComment(ctx context.Context, c *ArtifactComment) error {
	now := time.Now()
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO artifact_comments (kind, namespace, name, username, body, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`, c.Kind, c.Namespace, c.Name, c.Username, c.Body, now, now).Scan(&c.ID)
	if err != nil {
		return err
	}
	c.CreatedAt, c.UpdatedAt = now, now
	return nil
}

func (r *Repository) ListComments(ctx context.Context, kind, namespace, name string) ([]ArtifactComment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, kind, namespace, name, username, body, created_at, updated_at
		FROM artifact_comments WHERE kind = ? AND namespace = ? AND name = ?
		ORDER BY created_at ASC
	`, kind, namespace, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var comments []ArtifactComment
	for rows.Next() {
		var c ArtifactComment
		if err := rows.Scan(&c.ID, &c.Kind, &c.Namespace, &c.Name, &c.Username, &c.Body, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

func (r *Repository) UpdateComment(ctx context.Context, id int64, username, body string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE artifact_comments SET body = ?, updated_at = ? WHERE id = ? AND username = ?
	`, body, time.Now(), id, username)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("comment not found or not owned by you")
	}
	return nil
}

func (r *Repository) DeleteComment(ctx context.Context, id int64, username string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM artifact_comments WHERE id = ? AND username = ?`, id, username)
	return err
}

// ── Collections ────────────────────────────────────────────────────────────

type CollectionArtifact struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type Collection struct {
	ID          int64                `json:"id"`
	Owner       string               `json:"owner"`
	Slug        string               `json:"slug"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Artifacts   []CollectionArtifact `json:"artifacts"`
	Visibility  string               `json:"visibility"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
}

func (r *Repository) CreateCollection(ctx context.Context, c *Collection) error {
	c.CreatedAt, c.UpdatedAt = time.Now(), time.Now()
	arts, _ := json.Marshal(c.Artifacts)
	return r.db.QueryRowContext(ctx, `
		INSERT INTO collections (owner, slug, name, description, artifacts, visibility, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING id
	`, c.Owner, c.Slug, c.Name, c.Description, string(arts), c.Visibility, c.CreatedAt, c.UpdatedAt).Scan(&c.ID)
}

func (r *Repository) GetCollection(ctx context.Context, owner, slug string) (*Collection, error) {
	var c Collection
	var arts string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, owner, slug, name, description, artifacts, visibility, created_at, updated_at
		FROM collections WHERE owner = ? AND slug = ?
	`, owner, slug).Scan(&c.ID, &c.Owner, &c.Slug, &c.Name, &c.Description, &arts, &c.Visibility, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(arts), &c.Artifacts)
	return &c, nil
}

func (r *Repository) ListCollections(ctx context.Context, owner string) ([]Collection, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, owner, slug, name, description, artifacts, visibility, created_at, updated_at
		FROM collections WHERE owner = ? ORDER BY updated_at DESC
	`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Collection
	for rows.Next() {
		var c Collection
		var arts string
		if err := rows.Scan(&c.ID, &c.Owner, &c.Slug, &c.Name, &c.Description, &arts, &c.Visibility, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(arts), &c.Artifacts)
		result = append(result, c)
	}
	return result, rows.Err()
}

func (r *Repository) UpdateCollection(ctx context.Context, owner, slug string, name, description string, artifacts []CollectionArtifact, visibility string) error {
	arts, _ := json.Marshal(artifacts)
	_, err := r.db.ExecContext(ctx, `
		UPDATE collections SET name = ?, description = ?, artifacts = ?, visibility = ?, updated_at = ?
		WHERE owner = ? AND slug = ?
	`, name, description, string(arts), visibility, time.Now(), owner, slug)
	return err
}

func (r *Repository) DeleteCollection(ctx context.Context, owner, slug string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM collections WHERE owner = ? AND slug = ?`, owner, slug)
	return err
}

// ── Namespace analytics ───────────────────────────────────────────────────

type ArtifactInsight struct {
	Kind      string     `json:"kind"`
	Name      string     `json:"name"`
	Downloads int64      `json:"downloads"`
	Stars     int64      `json:"stars"`
	Trend     []DayCount `json:"trend"`
}

func (r *Repository) GetNamespaceInsights(ctx context.Context, namespace string, days int) ([]ArtifactInsight, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02")
	rows, err := r.db.QueryContext(ctx, `
		SELECT a.kind, a.name, a.id,
		       COALESCE((SELECT SUM(count) FROM artifact_download_counts WHERE artifact_id = a.id), 0) as downloads,
		       COALESCE((SELECT COUNT(*) FROM artifact_stars WHERE artifact_id = a.id), 0) as stars
		FROM artifacts a WHERE a.namespace = ?
		ORDER BY downloads DESC LIMIT 20
	`, namespace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ArtifactInsight
	for rows.Next() {
		var ins ArtifactInsight
		var id int64
		if err := rows.Scan(&ins.Kind, &ins.Name, &id, &ins.Downloads, &ins.Stars); err != nil {
			return nil, err
		}
		trendRows, tErr := r.db.QueryContext(ctx, `
			SELECT day, count FROM artifact_download_trend
			WHERE artifact_id = ? AND day >= ? ORDER BY day ASC
		`, id, cutoff)
		if tErr == nil {
			defer trendRows.Close()
			for trendRows.Next() {
				var dc DayCount
				_ = trendRows.Scan(&dc.Day, &dc.Count)
				ins.Trend = append(ins.Trend, dc)
			}
		}
		result = append(result, ins)
	}
	return result, rows.Err()
}

// ── Artifact transfer ─────────────────────────────────────────────────────

func (r *Repository) TransferArtifact(ctx context.Context, kind, fromNS, name, toNS string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE artifacts SET namespace = ?, updated_at = ? WHERE kind = ? AND namespace = ? AND name = ?
	`, toNS, time.Now(), kind, fromNS, name)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE artifact_versions SET namespace = ? WHERE kind = ? AND namespace = ? AND name = ?
	`, toNS, kind, fromNS, name)
	return err
}

// ── Follows ───────────────────────────────────────────────────────────────

func (r *Repository) FollowNamespace(ctx context.Context, follower, namespace string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO namespace_follows (follower, namespace, created_at) VALUES (?, ?, ?)`,
		follower, namespace, time.Now())
	return err
}

func (r *Repository) UnfollowNamespace(ctx context.Context, follower, namespace string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM namespace_follows WHERE follower = ? AND namespace = ?`, follower, namespace)
	return err
}

func (r *Repository) IsFollowing(ctx context.Context, follower, namespace string) bool {
	var c int
	_ = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM namespace_follows WHERE follower = ? AND namespace = ?`, follower, namespace).Scan(&c)
	return c > 0
}

func (r *Repository) GetFollowerCount(ctx context.Context, namespace string) int64 {
	var c int64
	_ = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM namespace_follows WHERE namespace = ?`, namespace).Scan(&c)
	return c
}

func (r *Repository) GetFollowing(ctx context.Context, follower string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT namespace FROM namespace_follows WHERE follower = ? ORDER BY created_at DESC`, follower)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var nss []string
	for rows.Next() {
		var ns string
		if err := rows.Scan(&ns); err != nil {
			return nil, err
		}
		nss = append(nss, ns)
	}
	return nss, rows.Err()
}
