package metadata

import (
	"time"
)

// Skill represents a skill in the registry
type Skill struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Namespace     string    `json:"namespace"`
	Description   string    `json:"description"`
	LatestVersion string    `json:"latest_version"`
	Visibility    string    `json:"visibility"` // public, private, internal
	Tags          []string  `json:"tags"`
	Owners        []string  `json:"owners"`
	Deprecated    bool      `json:"deprecated"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// SkillVersion represents a specific version of a skill
type SkillVersion struct {
	ID                  int64             `json:"id"`
	SkillID             int64             `json:"skill_id"`
	Name                string            `json:"name"`
	Namespace           string            `json:"namespace"`
	Version             string            `json:"version"`
	Description         string            `json:"description"`
	DigestSHA256        string            `json:"digest_sha256"`
	PackageType         string            `json:"package_type"` // zip, tgz
	SizeBytes           int64             `json:"size_bytes"`
	CreatedAt           time.Time         `json:"created_at"`
	CreatedBy           string            `json:"created_by"`
	RuntimeCompatibility map[string]string `json:"runtime_compatibility,omitempty"`
	AgentCompatibility   map[string]string `json:"agent_compatibility,omitempty"`
	EntrypointPath      string            `json:"entrypoint_path"`
	Manifest            *SkillManifest    `json:"manifest,omitempty"`
	ValidationStatus    string            `json:"validation_status"`
	SignatureStatus     string            `json:"signature_status"`
	Source              string            `json:"source"` // local, upstream
	Deprecated          bool              `json:"deprecated"`
}

// SkillManifest represents skill metadata
type SkillManifest struct {
	Name          string            `json:"name" yaml:"name"`
	Version       string            `json:"version" yaml:"version"`
	Description   string            `json:"description" yaml:"description"`
	Tags          []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	Compatibility map[string]string `json:"compatibility,omitempty" yaml:"compatibility,omitempty"`
	Entrypoint    string            `json:"entrypoint" yaml:"entrypoint"`
	License       string            `json:"license,omitempty" yaml:"license,omitempty"`
	Owners        []string          `json:"owners,omitempty" yaml:"owners,omitempty"`
}

// SkillList represents a paginated list of skills
type SkillList struct {
	Skills []Skill `json:"skills"`
	Total  int     `json:"total"`
	Limit  int     `json:"limit"`
	Offset int     `json:"offset"`
}

// VersionList represents a list of versions for a skill
type VersionList struct {
	Skill    Skill          `json:"skill"`
	Versions []SkillVersion `json:"versions"`
}

// AuditLog represents an audit log entry
type AuditLog struct {
	ID        int64     `json:"id"`
	Action    string    `json:"action"`
	Namespace string    `json:"namespace"`
	Name      string    `json:"name"`
	Version   string    `json:"version,omitempty"`
	Actor     string    `json:"actor"`
	Success   bool      `json:"success"`
	Message   string    `json:"message,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
