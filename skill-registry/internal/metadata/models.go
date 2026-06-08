package metadata

import (
	"time"

	"github.com/domehahn/sklib/spec"
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
	Downloads     int64     `json:"downloads"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// SkillVersion represents a specific version of a skill
type SkillVersion struct {
	ID                   int64             `json:"id"`
	SkillID              int64             `json:"skill_id"`
	Name                 string            `json:"name"`
	Namespace            string            `json:"namespace"`
	Version              string            `json:"version"`
	Description          string            `json:"description"`
	DigestSHA256         string            `json:"digest_sha256"`
	PackageType          string            `json:"package_type"` // zip, tgz
	SizeBytes            int64             `json:"size_bytes"`
	CreatedAt            time.Time         `json:"created_at"`
	CreatedBy            string            `json:"created_by"`
	RuntimeCompatibility map[string]string `json:"runtime_compatibility,omitempty"`
	AgentCompatibility   map[string]string `json:"agent_compatibility,omitempty"`
	EntrypointPath       string            `json:"entrypoint_path"`
	Manifest             *SkillManifest    `json:"manifest,omitempty"`
	ValidationStatus     string            `json:"validation_status"`
	SignatureStatus      string            `json:"signature_status"`
	SignatureDigest      string            `json:"signature_digest,omitempty"`
	Provenance           *Provenance       `json:"provenance,omitempty"`
	Source               string            `json:"source"` // local, upstream
	Deprecated           bool              `json:"deprecated"`
	DeprecationReason    string            `json:"deprecation_reason,omitempty"`
	Yanked               bool              `json:"yanked"`
	YankReason           string            `json:"yank_reason,omitempty"`
	StatusActor          string            `json:"status_actor,omitempty"`
	StatusUpdatedAt      *time.Time        `json:"status_updated_at,omitempty"`
	Downloads            int64             `json:"downloads"`
}

// DistTag maps a human-friendly channel such as latest or beta to a version.
type DistTag struct {
	Tag       string    `json:"tag"`
	Version   string    `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy string    `json:"updated_by"`
}

// SkillManifest represents skill metadata
type SkillManifest struct {
	Name           string            `json:"name" yaml:"name"`
	Namespace      string            `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Version        string            `json:"version" yaml:"version"`
	Description    string            `json:"description" yaml:"description"`
	Tags           []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	CompatibleWith []string          `json:"compatible_with,omitempty" yaml:"compatible_with,omitempty"`
	Compatibility  map[string]string `json:"compatibility,omitempty" yaml:"compatibility,omitempty"` // legacy input, normalized to CompatibleWith
	Entrypoint     string            `json:"entrypoint" yaml:"entrypoint"`
	License        string            `json:"license,omitempty" yaml:"license,omitempty"`
	Owners         []string          `json:"owners,omitempty" yaml:"owners,omitempty"`
	Security       SkillSecurity     `json:"security,omitempty" yaml:"security,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type SkillSecurity struct {
	RequiresNetwork bool `json:"requires_network" yaml:"requires_network"`
	RequiresSecrets bool `json:"requires_secrets" yaml:"requires_secrets"`
	WritesFiles     bool `json:"writes_files" yaml:"writes_files"`
	RunsCommands    bool `json:"runs_commands" yaml:"runs_commands"`
}

type Provenance struct {
	SourceRepository string    `json:"source_repository,omitempty" yaml:"source_repository,omitempty"`
	SourceCommit     string    `json:"source_commit,omitempty" yaml:"source_commit,omitempty"`
	BuilderIdentity  string    `json:"builder_identity,omitempty" yaml:"builder_identity,omitempty"`
	BuildTimestamp   time.Time `json:"build_timestamp,omitempty" yaml:"build_timestamp,omitempty"`
	PackageDigest    string    `json:"package_digest,omitempty" yaml:"package_digest,omitempty"`
	ToolVersion      string    `json:"tool_version,omitempty" yaml:"tool_version,omitempty"`
}

// SkillManifestFromSpec maps a spec.Skill (package metadata) to the local SkillManifest DB model.
func SkillManifestFromSpec(s *spec.Skill) *SkillManifest {
	if s == nil {
		return nil
	}
	return &SkillManifest{
		Name:           s.Name,
		Namespace:      s.Namespace,
		Version:        s.Version,
		Description:    s.Description,
		Tags:           s.Tags,
		CompatibleWith: spec.PlatformStrings(s.CompatibleWith),
		Entrypoint:     s.Entrypoint,
		License:        s.License,
		Owners:         s.Owners,
		Security: SkillSecurity{
			RequiresNetwork: s.Security.RequiresNetwork,
			RequiresSecrets: s.Security.RequiresSecrets,
			WritesFiles:     s.Security.WritesFiles,
			RunsCommands:    s.Security.RunsCommands,
		},
		Metadata: s.Metadata,
	}
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
