package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Repository manages skill metadata in SQLite
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new metadata repository
func NewRepository(dbPath string) (*Repository, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	repo := &Repository{db: db}
	if err := repo.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return repo, nil
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
	`

	_, err := r.db.Exec(schema)
	return err
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
			manifest, validation_status, signature_status, source, deprecated
		FROM skill_versions
		WHERE namespace = ? AND name = ? AND version = ?
	`

	var sv SkillVersion
	var runtimeJSON, agentJSON, manifestJSON string
	var deprecated int

	err := r.db.QueryRowContext(ctx, query, namespace, name, version).Scan(
		&sv.ID, &sv.SkillID, &sv.Name, &sv.Namespace, &sv.Version, &sv.Description,
		&sv.DigestSHA256, &sv.PackageType, &sv.SizeBytes, &sv.CreatedAt, &sv.CreatedBy,
		&runtimeJSON, &agentJSON, &sv.EntrypointPath, &manifestJSON,
		&sv.ValidationStatus, &sv.SignatureStatus, &sv.Source, &deprecated,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	sv.Deprecated = intToBool(deprecated)
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
			manifest, validation_status, signature_status, source, deprecated
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

		err := rows.Scan(
			&sv.ID, &sv.SkillID, &sv.Name, &sv.Namespace, &sv.Version, &sv.Description,
			&sv.DigestSHA256, &sv.PackageType, &sv.SizeBytes, &sv.CreatedAt, &sv.CreatedBy,
			&runtimeJSON, &agentJSON, &sv.EntrypointPath, &manifestJSON,
			&sv.ValidationStatus, &sv.SignatureStatus, &sv.Source, &deprecated,
		)
		if err != nil {
			return nil, err
		}

		sv.Deprecated = intToBool(deprecated)
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

// HardDeleteVersion removes a version completely from the database
func (r *Repository) HardDeleteVersion(ctx context.Context, namespace, name, version string) error {
	query := `DELETE FROM skill_versions WHERE namespace = ? AND name = ? AND version = ?`
	_, err := r.db.ExecContext(ctx, query, namespace, name, version)
	return err
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

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(i int) bool {
	return i != 0
}
