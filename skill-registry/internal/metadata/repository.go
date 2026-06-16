package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/skillforge/skill-registry/internal/database"
	"github.com/skillforge/skill-registry/internal/dbmigrate"
)

// Repository manages skill metadata.
type Repository struct {
	db     *sql.DB
	driver string
}

// NewRepository creates a new metadata repository
func NewRepository(dbPath string) (*Repository, error) {
	return NewRepositoryWithConfig(database.DriverSQLite, dbPath)
}

// NewRepositoryWithConfig creates a repository for the selected database backend.
func NewRepositoryWithConfig(driver, dsn string) (*Repository, error) {
	db, err := database.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	repo := &Repository{db: db, driver: driver}
	if err := repo.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return repo, nil
}

// Ping checks that the database connection is alive.
func (r *Repository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

// Close closes the database connection
func (r *Repository) Close() error {
	return r.db.Close()
}

// GetDB returns the database connection
func (r *Repository) GetDB() *sql.DB {
	return r.db
}

func (r *Repository) migrate() error {
	if err := dbmigrate.Run(r.db, "meta_001_initial", func(tx *sql.Tx) error {
		return r.applyInitialSchema(tx)
	}); err != nil {
		return err
	}
	if err := dbmigrate.Run(r.db, "meta_002_artifact_readme", func(tx *sql.Tx) error {
		_, err := tx.Exec(`ALTER TABLE artifacts ADD COLUMN readme TEXT NOT NULL DEFAULT ''`)
		return err
	}); err != nil {
		return err
	}
	if err := dbmigrate.Run(r.db, "meta_003_namespace_verified", func(tx *sql.Tx) error {
		// Ensure namespace_profiles exists (may not be present in DBs predating its addition to initial schema)
		if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS namespace_profiles (
			namespace TEXT PRIMARY KEY,
			bio TEXT NOT NULL DEFAULT '',
			updated_at DATETIME NOT NULL
		)`); err != nil {
			return err
		}
		_, err := tx.Exec(`ALTER TABLE namespace_profiles ADD COLUMN verified INTEGER NOT NULL DEFAULT 0`)
		return err
	}); err != nil {
		return err
	}
	if err := dbmigrate.Run(r.db, "meta_004_display_name", func(tx *sql.Tx) error {
		_, err := tx.Exec(`ALTER TABLE namespace_profiles ADD COLUMN display_name TEXT NOT NULL DEFAULT ''`)
		return err
	}); err != nil {
		return err
	}
	if err := dbmigrate.Run(r.db, "meta_005_download_trend", func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS artifact_download_trend (
			artifact_id INTEGER NOT NULL,
			day TEXT NOT NULL,
			count INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (artifact_id, day)
		)`)
		return err
	}); err != nil {
		return err
	}
	if err := dbmigrate.Run(r.db, "meta_006_version_notes", func(tx *sql.Tx) error {
		_, err := tx.Exec(`ALTER TABLE artifact_versions ADD COLUMN release_notes TEXT NOT NULL DEFAULT ''`)
		return err
	}); err != nil {
		return err
	}
	if err := dbmigrate.Run(r.db, "meta_007_webhook_deliveries", func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS webhook_deliveries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			webhook_id INTEGER NOT NULL,
			namespace TEXT NOT NULL,
			event TEXT NOT NULL DEFAULT 'test',
			status_code INTEGER NOT NULL DEFAULT 0,
			duration_ms INTEGER NOT NULL DEFAULT 0,
			success INTEGER NOT NULL DEFAULT 0,
			delivered_at DATETIME NOT NULL
		)`)
		return err
	}); err != nil {
		return err
	}
	if err := dbmigrate.Run(r.db, "meta_008_pinned", func(tx *sql.Tx) error {
		_, err := tx.Exec(`ALTER TABLE namespace_profiles ADD COLUMN pinned TEXT NOT NULL DEFAULT '[]'`)
		return err
	}); err != nil {
		return err
	}
	if err := dbmigrate.Run(r.db, "meta_009_notifications", func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS notifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT 'info',
			message TEXT NOT NULL,
			link TEXT NOT NULL DEFAULT '',
			read INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL
		)`)
		return err
	}); err != nil {
		return err
	}
	if err := dbmigrate.Run(r.db, "meta_010_stars", func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS artifact_stars (
			artifact_id INTEGER NOT NULL,
			username TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			PRIMARY KEY (artifact_id, username)
		)`)
		return err
	}); err != nil {
		return err
	}
	if err := dbmigrate.Run(r.db, "meta_011_scan_results", func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS scan_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			artifact_version_id INTEGER NOT NULL,
			severity TEXT NOT NULL DEFAULT 'unknown',
			cve_id TEXT NOT NULL DEFAULT '',
			package_name TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			fixed_version TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL
		)`)
		return err
	}); err != nil {
		return err
	}
	if err := dbmigrate.Run(r.db, "meta_012_comments", func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS artifact_comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			kind TEXT NOT NULL,
			namespace TEXT NOT NULL,
			name TEXT NOT NULL,
			username TEXT NOT NULL,
			body TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`)
		return err
	}); err != nil {
		return err
	}
	if err := dbmigrate.Run(r.db, "meta_013_collections", func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS collections (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			owner TEXT NOT NULL,
			slug TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			artifacts TEXT NOT NULL DEFAULT '[]',
			visibility TEXT NOT NULL DEFAULT 'public',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			UNIQUE(owner, slug)
		)`)
		return err
	}); err != nil {
		return err
	}
	return dbmigrate.Run(r.db, "meta_014_follows", func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS namespace_follows (
			follower TEXT NOT NULL,
			namespace TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			PRIMARY KEY (follower, namespace)
		)`)
		return err
	})
}

func (r *Repository) applyInitialSchema(tx *sql.Tx) error {
	schema := `
	CREATE TABLE IF NOT EXISTS skills (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		namespace TEXT NOT NULL,
		description TEXT,
		latest_version TEXT,
		visibility TEXT DEFAULT 'public',
		tags TEXT, -- JSON array
		owners TEXT, -- JSON array
		deprecated INTEGER DEFAULT 0,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		UNIQUE(namespace, name)
	);

	CREATE INDEX IF NOT EXISTS idx_skills_namespace ON skills(namespace);
	CREATE INDEX IF NOT EXISTS idx_skills_name ON skills(name);

	CREATE TABLE IF NOT EXISTS skill_versions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		skill_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		namespace TEXT NOT NULL,
		version TEXT NOT NULL,
		description TEXT,
		digest_sha256 TEXT NOT NULL,
		package_type TEXT NOT NULL,
		size_bytes INTEGER NOT NULL,
		created_at DATETIME NOT NULL,
		created_by TEXT,
		runtime_compatibility TEXT, -- JSON object
		agent_compatibility TEXT, -- JSON object
		entrypoint_path TEXT,
		manifest TEXT, -- JSON object
		validation_status TEXT,
		signature_status TEXT,
		source TEXT DEFAULT 'local',
		deprecated INTEGER DEFAULT 0,
		FOREIGN KEY (skill_id) REFERENCES skills(id),
		UNIQUE(skill_id, version)
	);

	CREATE INDEX IF NOT EXISTS idx_versions_skill_id ON skill_versions(skill_id);
	CREATE INDEX IF NOT EXISTS idx_versions_digest ON skill_versions(digest_sha256);

	CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		action TEXT NOT NULL,
		namespace TEXT NOT NULL,
		name TEXT NOT NULL,
		version TEXT,
		actor TEXT,
		success INTEGER NOT NULL,
		message TEXT,
		created_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_audit_created_at ON audit_log(created_at);

	CREATE TABLE IF NOT EXISTS dist_tags (
		skill_id INTEGER NOT NULL,
		tag TEXT NOT NULL,
		version TEXT NOT NULL,
		updated_at DATETIME NOT NULL,
		updated_by TEXT,
		FOREIGN KEY (skill_id) REFERENCES skills(id),
		UNIQUE(skill_id, tag)
	);

	CREATE TABLE IF NOT EXISTS download_counts (
		namespace TEXT NOT NULL,
		name TEXT NOT NULL,
		version TEXT NOT NULL,
		count INTEGER NOT NULL DEFAULT 0,
		updated_at DATETIME NOT NULL,
		UNIQUE(namespace, name, version)
	);

	INSERT OR IGNORE INTO dist_tags (skill_id, tag, version, updated_at, updated_by)
	SELECT id, 'latest', latest_version, updated_at, 'migration'
	FROM skills
	WHERE latest_version IS NOT NULL AND latest_version != '';

	CREATE TABLE IF NOT EXISTS artifacts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		kind TEXT NOT NULL,
		namespace TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		latest_version TEXT,
		visibility TEXT NOT NULL DEFAULT 'public',
		tags TEXT,
		owners TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		UNIQUE(kind, namespace, name)
	);

	CREATE TABLE IF NOT EXISTS artifact_versions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		artifact_id INTEGER NOT NULL,
		kind TEXT NOT NULL,
		namespace TEXT NOT NULL,
		name TEXT NOT NULL,
		version TEXT NOT NULL,
		digest_sha256 TEXT NOT NULL,
		package_type TEXT NOT NULL,
		size_bytes INTEGER NOT NULL,
		entrypoint TEXT,
		manifest TEXT NOT NULL,
		lockfile TEXT,
		oci_descriptor TEXT NOT NULL,
		signature_status TEXT NOT NULL DEFAULT 'unsigned',
		scan_status TEXT NOT NULL DEFAULT 'pending',
		validation_status TEXT NOT NULL DEFAULT 'valid',
		source TEXT NOT NULL DEFAULT 'local',
		created_by TEXT,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (artifact_id) REFERENCES artifacts(id),
		UNIQUE(artifact_id, version)
	);

	CREATE TABLE IF NOT EXISTS artifact_dist_tags (
		artifact_id INTEGER NOT NULL,
		tag TEXT NOT NULL,
		version TEXT NOT NULL,
		updated_at DATETIME NOT NULL,
		updated_by TEXT,
		FOREIGN KEY (artifact_id) REFERENCES artifacts(id),
		UNIQUE(artifact_id, tag)
	);

	CREATE TABLE IF NOT EXISTS artifact_download_counts (
		artifact_id INTEGER NOT NULL,
		version TEXT NOT NULL,
		count INTEGER NOT NULL DEFAULT 0,
		updated_at DATETIME NOT NULL,
		UNIQUE(artifact_id, version)
	);

	CREATE TABLE IF NOT EXISTS promotions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		kind TEXT NOT NULL,
		namespace TEXT NOT NULL,
		name TEXT NOT NULL,
		version TEXT NOT NULL,
		channel TEXT NOT NULL,
		actor TEXT,
		created_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS namespace_members (
		namespace TEXT NOT NULL,
		username TEXT NOT NULL,
		role TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		UNIQUE(namespace, username)
	);

	CREATE TABLE IF NOT EXISTS webhooks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		namespace TEXT NOT NULL,
		url TEXT NOT NULL,
		events TEXT NOT NULL,
		active INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS attestations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		artifact_version_id INTEGER NOT NULL,
		type TEXT NOT NULL,
		digest TEXT NOT NULL,
		predicate TEXT,
		created_by TEXT,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (artifact_version_id) REFERENCES artifact_versions(id)
	);

	CREATE TABLE IF NOT EXISTS namespace_profiles (
		namespace TEXT PRIMARY KEY,
		bio TEXT NOT NULL DEFAULT '',
		updated_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_artifacts_lookup ON artifacts(kind, namespace, name);
	CREATE INDEX IF NOT EXISTS idx_artifact_versions_lookup ON artifact_versions(kind, namespace, name, version);

	INSERT OR IGNORE INTO artifacts (kind, namespace, name, description, latest_version, visibility, tags, owners, created_at, updated_at)
	SELECT 'skill', namespace, name, description, latest_version, visibility, tags, owners, created_at, updated_at FROM skills;

	INSERT OR IGNORE INTO artifact_versions (
		artifact_id, kind, namespace, name, version, digest_sha256, package_type, size_bytes,
		entrypoint, manifest, lockfile, oci_descriptor, signature_status, scan_status,
		validation_status, source, created_by, created_at
	)
	SELECT a.id, 'skill', sv.namespace, sv.name, sv.version, sv.digest_sha256, sv.package_type, sv.size_bytes,
		sv.entrypoint_path,
		json_object(
			'apiVersion', 'skillforge.dev/v1', 'kind', 'skill',
			'metadata', json_object('namespace', sv.namespace, 'name', sv.name, 'version', sv.version, 'description', sv.description),
			'spec', json_object('entrypoint', sv.entrypoint_path, 'dependencies', json_array())
		),
		json_object('apiVersion', 'skillforge.dev/v1', 'root', 'skill/' || sv.namespace || '/' || sv.name || '@' || sv.version, 'resolved', json_array()),
		json_object('mediaType', 'application/vnd.skillforge.artifact.manifest.v1+json', 'digest', 'sha256:' || sv.digest_sha256, 'size', sv.size_bytes, 'artifactType', 'application/vnd.skillforge.skill.v1'),
		sv.signature_status, 'pending', sv.validation_status, sv.source, sv.created_by, sv.created_at
	FROM skill_versions sv
	JOIN artifacts a ON a.kind = 'skill' AND a.namespace = sv.namespace AND a.name = sv.name;

	INSERT OR IGNORE INTO artifact_dist_tags (artifact_id, tag, version, updated_at, updated_by)
	SELECT a.id, dt.tag, dt.version, dt.updated_at, dt.updated_by
	FROM dist_tags dt JOIN skills s ON s.id = dt.skill_id
	JOIN artifacts a ON a.kind = 'skill' AND a.namespace = s.namespace AND a.name = s.name;
	`

	if _, err := tx.Exec(schema); err != nil {
		return err
	}
	for _, stmt := range []string{
		`ALTER TABLE skill_versions ADD COLUMN deprecation_reason TEXT DEFAULT ''`,
		`ALTER TABLE skill_versions ADD COLUMN yanked INTEGER DEFAULT 0`,
		`ALTER TABLE skill_versions ADD COLUMN yank_reason TEXT DEFAULT ''`,
		`ALTER TABLE skill_versions ADD COLUMN status_actor TEXT DEFAULT ''`,
		`ALTER TABLE skill_versions ADD COLUMN status_updated_at DATETIME`,
	} {
		if _, err := tx.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return err
		}
	}
	return nil
}

// CreateOrUpdateSkill creates or updates a skill
func (r *Repository) CreateOrUpdateSkill(ctx context.Context, skill *Skill) error {
	tagsJSON, _ := json.Marshal(skill.Tags)
	ownersJSON, _ := json.Marshal(skill.Owners)

	now := time.Now()
	if skill.CreatedAt.IsZero() {
		skill.CreatedAt = now
	}
	skill.UpdatedAt = now

	query := `
		INSERT INTO skills (name, namespace, description, latest_version, visibility, tags, owners, deprecated, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(namespace, name) DO UPDATE SET
			description = excluded.description,
			latest_version = excluded.latest_version,
			visibility = excluded.visibility,
			tags = excluded.tags,
			owners = excluded.owners,
			deprecated = excluded.deprecated,
			updated_at = excluded.updated_at
		RETURNING id
	`

	err := r.db.QueryRowContext(ctx, query,
		skill.Name, skill.Namespace, skill.Description, skill.LatestVersion,
		skill.Visibility, string(tagsJSON), string(ownersJSON),
		boolToInt(skill.Deprecated), skill.CreatedAt, skill.UpdatedAt,
	).Scan(&skill.ID)

	return err
}

// GetSkill retrieves a skill by namespace and name
func (r *Repository) GetSkill(ctx context.Context, namespace, name string) (*Skill, error) {
	query := `
		SELECT id, name, namespace, description, latest_version, visibility, tags, owners, deprecated, created_at, updated_at
		FROM skills
		WHERE namespace = ? AND name = ?
	`

	var skill Skill
	var tagsJSON, ownersJSON string
	var deprecated int

	err := r.db.QueryRowContext(ctx, query, namespace, name).Scan(
		&skill.ID, &skill.Name, &skill.Namespace, &skill.Description, &skill.LatestVersion,
		&skill.Visibility, &tagsJSON, &ownersJSON, &deprecated,
		&skill.CreatedAt, &skill.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	skill.Deprecated = intToBool(deprecated)
	json.Unmarshal([]byte(tagsJSON), &skill.Tags)
	json.Unmarshal([]byte(ownersJSON), &skill.Owners)

	return &skill, nil
}

// ListSkills lists skills with optional filters
func (r *Repository) ListSkills(ctx context.Context, filters map[string]interface{}, limit, offset int) (*SkillList, error) {
	var conditions []string
	var args []interface{}

	if q, ok := filters["q"].(string); ok && q != "" {
		conditions = append(conditions, "(name LIKE ? OR description LIKE ?)")
		args = append(args, "%"+q+"%", "%"+q+"%")
	}
	if ns, ok := filters["namespace"].(string); ok && ns != "" {
		conditions = append(conditions, "namespace = ?")
		args = append(args, ns)
	}
	if tag, ok := filters["tag"].(string); ok && tag != "" {
		conditions = append(conditions, `EXISTS (SELECT 1 FROM json_each(skills.tags) WHERE json_each.value = ?)`)
		args = append(args, tag)
	}
	if deprecated, ok := filters["deprecated"].(bool); ok {
		conditions = append(conditions, "deprecated = ?")
		args = append(args, boolToInt(deprecated))
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM skills %s", whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	// Fetch skills
	query := fmt.Sprintf(`
		SELECT id, name, namespace, description, latest_version, visibility, tags, owners, deprecated, created_at, updated_at
		FROM skills
		%s
		ORDER BY updated_at DESC
		LIMIT ? OFFSET ?
	`, whereClause)

	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var skills []Skill
	for rows.Next() {
		var skill Skill
		var tagsJSON, ownersJSON string
		var deprecated int

		err := rows.Scan(
			&skill.ID, &skill.Name, &skill.Namespace, &skill.Description, &skill.LatestVersion,
			&skill.Visibility, &tagsJSON, &ownersJSON, &deprecated,
			&skill.CreatedAt, &skill.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		skill.Deprecated = intToBool(deprecated)
		json.Unmarshal([]byte(tagsJSON), &skill.Tags)
		json.Unmarshal([]byte(ownersJSON), &skill.Owners)
		skills = append(skills, skill)
	}

	return &SkillList{
		Skills: skills,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

// CreateVersion creates a new skill version
func (r *Repository) CreateVersion(ctx context.Context, version *SkillVersion) error {
	runtimeJSON, _ := json.Marshal(version.RuntimeCompatibility)
	agentJSON, _ := json.Marshal(version.AgentCompatibility)
	manifestJSON, _ := json.Marshal(version.Manifest)

	if version.CreatedAt.IsZero() {
		version.CreatedAt = time.Now()
	}

	query := `
		INSERT INTO skill_versions (
			skill_id, name, namespace, version, description, digest_sha256, package_type, size_bytes,
			created_at, created_by, runtime_compatibility, agent_compatibility, entrypoint_path,
			manifest, validation_status, signature_status, source, deprecated
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`

	err := r.db.QueryRowContext(ctx, query,
		version.SkillID, version.Name, version.Namespace, version.Version, version.Description,
		version.DigestSHA256, version.PackageType, version.SizeBytes, version.CreatedAt, version.CreatedBy,
		string(runtimeJSON), string(agentJSON), version.EntrypointPath, string(manifestJSON),
		version.ValidationStatus, version.SignatureStatus, version.Source, boolToInt(version.Deprecated),
	).Scan(&version.ID)

	return err
}

// GetVersion retrieves a specific version
func (r *Repository) GetVersion(ctx context.Context, namespace, name, version string) (*SkillVersion, error) {
	query := `
		SELECT id, skill_id, name, namespace, version, description, digest_sha256, package_type, size_bytes,
			created_at, created_by, runtime_compatibility, agent_compatibility, entrypoint_path,
			manifest, validation_status, signature_status, source, deprecated,
			COALESCE(deprecation_reason, ''), COALESCE(yanked, 0), COALESCE(yank_reason, ''),
			COALESCE(status_actor, ''), status_updated_at
		FROM skill_versions
		WHERE namespace = ? AND name = ? AND version = ?
	`

	var sv SkillVersion
	var runtimeJSON, agentJSON, manifestJSON string
	var deprecated int
	var yanked int
	var statusUpdatedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, namespace, name, version).Scan(
		&sv.ID, &sv.SkillID, &sv.Name, &sv.Namespace, &sv.Version, &sv.Description,
		&sv.DigestSHA256, &sv.PackageType, &sv.SizeBytes, &sv.CreatedAt, &sv.CreatedBy,
		&runtimeJSON, &agentJSON, &sv.EntrypointPath, &manifestJSON,
		&sv.ValidationStatus, &sv.SignatureStatus, &sv.Source, &deprecated,
		&sv.DeprecationReason, &yanked, &sv.YankReason, &sv.StatusActor, &statusUpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	sv.Deprecated = intToBool(deprecated)
	sv.Yanked = intToBool(yanked)
	if statusUpdatedAt.Valid {
		sv.StatusUpdatedAt = &statusUpdatedAt.Time
	}
	json.Unmarshal([]byte(runtimeJSON), &sv.RuntimeCompatibility)
	json.Unmarshal([]byte(agentJSON), &sv.AgentCompatibility)
	json.Unmarshal([]byte(manifestJSON), &sv.Manifest)

	return &sv, nil
}

// ListVersions lists all versions for a skill
func (r *Repository) ListVersions(ctx context.Context, skillID int64) ([]SkillVersion, error) {
	query := `
		SELECT id, skill_id, name, namespace, version, description, digest_sha256, package_type, size_bytes,
			created_at, created_by, runtime_compatibility, agent_compatibility, entrypoint_path,
			manifest, validation_status, signature_status, source, deprecated,
			COALESCE(deprecation_reason, ''), COALESCE(yanked, 0), COALESCE(yank_reason, ''),
			COALESCE(status_actor, ''), status_updated_at
		FROM skill_versions
		WHERE skill_id = ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, skillID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []SkillVersion
	for rows.Next() {
		var sv SkillVersion
		var runtimeJSON, agentJSON, manifestJSON string
		var deprecated int
		var yanked int
		var statusUpdatedAt sql.NullTime

		err := rows.Scan(
			&sv.ID, &sv.SkillID, &sv.Name, &sv.Namespace, &sv.Version, &sv.Description,
			&sv.DigestSHA256, &sv.PackageType, &sv.SizeBytes, &sv.CreatedAt, &sv.CreatedBy,
			&runtimeJSON, &agentJSON, &sv.EntrypointPath, &manifestJSON,
			&sv.ValidationStatus, &sv.SignatureStatus, &sv.Source, &deprecated,
			&sv.DeprecationReason, &yanked, &sv.YankReason, &sv.StatusActor, &statusUpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		sv.Deprecated = intToBool(deprecated)
		sv.Yanked = intToBool(yanked)
		if statusUpdatedAt.Valid {
			sv.StatusUpdatedAt = &statusUpdatedAt.Time
		}
		json.Unmarshal([]byte(runtimeJSON), &sv.RuntimeCompatibility)
		json.Unmarshal([]byte(agentJSON), &sv.AgentCompatibility)
		json.Unmarshal([]byte(manifestJSON), &sv.Manifest)
		versions = append(versions, sv)
	}

	return versions, nil
}

// DeleteVersion soft-deletes a version
func (r *Repository) DeleteVersion(ctx context.Context, namespace, name, version string) error {
	query := `UPDATE skill_versions SET deprecated = 1 WHERE namespace = ? AND name = ? AND version = ?`
	_, err := r.db.ExecContext(ctx, query, namespace, name, version)
	return err
}

func (r *Repository) DeprecateVersion(ctx context.Context, namespace, name, version, reason, actor string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE skill_versions
		SET deprecated = 1, deprecation_reason = ?, status_actor = ?, status_updated_at = ?
		WHERE namespace = ? AND name = ? AND version = ?
	`, reason, actor, time.Now(), namespace, name, version)
	return err
}

func (r *Repository) YankVersion(ctx context.Context, namespace, name, version, reason, actor string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE skill_versions
		SET yanked = 1, yank_reason = ?, status_actor = ?, status_updated_at = ?
		WHERE namespace = ? AND name = ? AND version = ?
	`, reason, actor, time.Now(), namespace, name, version)
	return err
}

func (r *Repository) UnyankVersion(ctx context.Context, namespace, name, version, actor string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE skill_versions
		SET yanked = 0, yank_reason = '', status_actor = ?, status_updated_at = ?
		WHERE namespace = ? AND name = ? AND version = ?
	`, actor, time.Now(), namespace, name, version)
	return err
}

// HardDeleteVersion removes a version completely from the database
func (r *Repository) HardDeleteVersion(ctx context.Context, namespace, name, version string) error {
	query := `DELETE FROM skill_versions WHERE namespace = ? AND name = ? AND version = ?`
	_, err := r.db.ExecContext(ctx, query, namespace, name, version)
	return err
}

// SetDistTag assigns a channel tag to an existing version.
func (r *Repository) SetDistTag(ctx context.Context, namespace, name, tag, version, actor string) error {
	skill, err := r.GetSkill(ctx, namespace, name)
	if err != nil {
		return err
	}
	if skill == nil {
		return fmt.Errorf("skill not found")
	}
	sv, err := r.GetVersion(ctx, namespace, name, version)
	if err != nil {
		return err
	}
	if sv == nil {
		return fmt.Errorf("version not found")
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO dist_tags (skill_id, tag, version, updated_at, updated_by)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(skill_id, tag) DO UPDATE SET
			version = excluded.version,
			updated_at = excluded.updated_at,
			updated_by = excluded.updated_by
	`, skill.ID, tag, version, time.Now(), actor)
	if err != nil {
		return err
	}
	if tag == "latest" {
		_, err = r.db.ExecContext(ctx, `UPDATE skills SET latest_version = ?, updated_at = ? WHERE id = ?`, version, time.Now(), skill.ID)
	}
	return err
}

// ListDistTags returns all channel tags for a skill.
func (r *Repository) ListDistTags(ctx context.Context, namespace, name string) (map[string]string, error) {
	skill, err := r.GetSkill(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	if skill == nil {
		return nil, fmt.Errorf("skill not found")
	}

	rows, err := r.db.QueryContext(ctx, `SELECT tag, version FROM dist_tags WHERE skill_id = ? ORDER BY tag`, skill.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make(map[string]string)
	for rows.Next() {
		var tag, version string
		if err := rows.Scan(&tag, &version); err != nil {
			return nil, err
		}
		tags[tag] = version
	}
	return tags, rows.Err()
}

// IncrementDownload records a successful package pull.
func (r *Repository) IncrementDownload(ctx context.Context, namespace, name, version string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO download_counts (namespace, name, version, count, updated_at)
		VALUES (?, ?, ?, 1, ?)
		ON CONFLICT(namespace, name, version) DO UPDATE SET
			count = count + 1,
			updated_at = excluded.updated_at
	`, namespace, name, version, time.Now())
	return err
}

// DownloadCount returns pulls for a version, or all versions when version is empty.
func (r *Repository) DownloadCount(ctx context.Context, namespace, name, version string) (int64, error) {
	query := `SELECT COALESCE(SUM(count), 0) FROM download_counts WHERE namespace = ? AND name = ?`
	args := []interface{}{namespace, name}
	if version != "" {
		query += ` AND version = ?`
		args = append(args, version)
	}
	var count int64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

// LogAudit creates an audit log entry
func (r *Repository) LogAudit(ctx context.Context, log *AuditLog) error {
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}

	query := `
		INSERT INTO audit_log (action, namespace, name, version, actor, success, message, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`

	err := r.db.QueryRowContext(ctx, query,
		log.Action, log.Namespace, log.Name, log.Version, log.Actor,
		boolToInt(log.Success), log.Message, log.CreatedAt,
	).Scan(&log.ID)

	return err
}

// MetricsSnapshot holds registry-wide counters for the /metrics and /stats endpoints.
type MetricsSnapshot struct {
	TotalSkills        int64 `json:"total_skills"`
	TotalVersions      int64 `json:"total_versions"`
	ActiveVersions     int64 `json:"active_versions"`
	YankedVersions     int64 `json:"yanked_versions"`
	DeprecatedVersions int64 `json:"deprecated_versions"`
	TotalDownloads     int64 `json:"total_downloads"`
	ActiveTokens       int64 `json:"active_tokens"`
	// Artifact-level aggregates (used by the web UI stats bar)
	TotalArtifacts  int64 `json:"total_artifacts"`
	TotalNamespaces int64 `json:"total_namespaces"`
}

// GetMetricsSnapshot queries key counters from the database.
func (r *Repository) GetMetricsSnapshot(ctx context.Context) (*MetricsSnapshot, error) {
	m := &MetricsSnapshot{}
	queries := []struct {
		dest *int64
		q    string
	}{
		{&m.TotalSkills, `SELECT COUNT(*) FROM skills`},
		{&m.TotalVersions, `SELECT COUNT(*) FROM skill_versions`},
		{&m.ActiveVersions, `SELECT COUNT(*) FROM skill_versions WHERE yanked = 0 AND deprecated = 0`},
		{&m.YankedVersions, `SELECT COUNT(*) FROM skill_versions WHERE yanked = 1`},
		{&m.DeprecatedVersions, `SELECT COUNT(*) FROM skill_versions WHERE deprecated = 1`},
		{&m.TotalDownloads, `SELECT COALESCE(SUM(count), 0) FROM download_counts`},
		{&m.ActiveTokens, `SELECT COUNT(*) FROM tokens WHERE revoked_at IS NULL`},
		{&m.TotalArtifacts, `SELECT COUNT(*) FROM artifacts`},
		{&m.TotalNamespaces, `SELECT COUNT(DISTINCT namespace) FROM artifacts`},
	}
	for _, row := range queries {
		if err := r.db.QueryRowContext(ctx, row.q).Scan(row.dest); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(i int) bool {
	return i != 0
}
