package registry

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/skillforge/skill-registry/internal/metadata"
	"github.com/skillforge/skill-registry/internal/storage"
	"github.com/skillforge/skill-registry/internal/validation"
)

// Registry orchestrates skill publishing and retrieval
type Registry struct {
	repo      *metadata.Repository
	storage   *storage.Storage
	validator *validation.Validator
	logger    *slog.Logger
}

// NewRegistry creates a new registry instance
func NewRegistry(repo *metadata.Repository, storage *storage.Storage, validator *validation.Validator, logger *slog.Logger) *Registry {
	return &Registry{
		repo:      repo,
		storage:   storage,
		validator: validator,
		logger:    logger,
	}
}

// PublishOptions contains options for publishing a skill
type PublishOptions struct {
	Force     bool
	CreatedBy string
	Source    string
}

// Publish publishes a skill package
func (r *Registry) Publish(ctx context.Context, namespace, name, version string, data []byte, packageType string, opts PublishOptions) (*metadata.SkillVersion, error) {
	// Validate name and version
	if err := validation.ValidateSkillName(name); err != nil {
		return nil, err
	}
	if err := validation.ValidateVersion(version); err != nil {
		return nil, err
	}

	// Validate package
	result, err := r.validator.ValidatePackage(data, packageType)
	if err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	if !result.Valid {
		return nil, fmt.Errorf("package validation failed: %s", strings.Join(result.Errors, "; "))
	}

	// Check if version already exists
	existing, err := r.repo.GetVersion(ctx, namespace, name, version)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing version: %w", err)
	}
	if existing != nil {
		if !opts.Force {
			return nil, fmt.Errorf("version %s already exists (use force to overwrite)", version)
		}
		// Hard delete existing version when force is true to avoid UNIQUE constraint
		if err := r.repo.HardDeleteVersion(ctx, namespace, name, version); err != nil {
			return nil, fmt.Errorf("failed to delete existing version: %w", err)
		}
	}

	// Store package
	digest, err := r.storage.Store(namespace, name, version, data)
	if err != nil {
		return nil, fmt.Errorf("failed to store package: %w", err)
	}

	// Create or update skill
	skill := &metadata.Skill{
		Name:          name,
		Namespace:     namespace,
		LatestVersion: version,
		Visibility:    "public",
	}

	if result.Metadata != nil {
		skill.Description = result.Metadata.Description
		skill.Tags = result.Metadata.Tags
		skill.Owners = result.Metadata.Owners
	}

	if err := r.repo.CreateOrUpdateSkill(ctx, skill); err != nil {
		return nil, fmt.Errorf("failed to create skill: %w", err)
	}

	// Create version
	skillVersion := &metadata.SkillVersion{
		SkillID:          skill.ID,
		Name:             name,
		Namespace:        namespace,
		Version:          version,
		Description:      skill.Description,
		DigestSHA256:     digest,
		PackageType:      packageType,
		SizeBytes:        int64(len(data)),
		CreatedAt:        time.Now(),
		CreatedBy:        opts.CreatedBy,
		EntrypointPath:   "SKILL.md",
		Manifest:         result.Metadata,
		ValidationStatus: "valid",
		SignatureStatus:  "unsigned",
		Source:           "local",
	}

	if opts.Source != "" {
		skillVersion.Source = opts.Source
	}

	if result.Metadata != nil {
		skillVersion.EntrypointPath = result.Metadata.Entrypoint
		skillVersion.AgentCompatibility = result.Metadata.Compatibility
	}

	if err := r.repo.CreateVersion(ctx, skillVersion); err != nil {
		return nil, fmt.Errorf("failed to create version: %w", err)
	}

	// Log audit entry
	auditLog := &metadata.AuditLog{
		Action:    "publish",
		Namespace: namespace,
		Name:      name,
		Version:   version,
		Actor:     opts.CreatedBy,
		Success:   true,
	}
	r.repo.LogAudit(ctx, auditLog)

	r.logger.Info("skill published",
		"namespace", namespace,
		"name", name,
		"version", version,
		"digest", digest,
	)

	return skillVersion, nil
}

// GetSkill retrieves a skill with its versions
func (r *Registry) GetSkill(ctx context.Context, namespace, name string) (*metadata.Skill, []metadata.SkillVersion, error) {
	skill, err := r.repo.GetSkill(ctx, namespace, name)
	if err != nil {
		return nil, nil, err
	}
	if skill == nil {
		return nil, nil, fmt.Errorf("skill not found")
	}

	versions, err := r.repo.ListVersions(ctx, skill.ID)
	if err != nil {
		return nil, nil, err
	}

	return skill, versions, nil
}

// GetVersion retrieves a specific version
func (r *Registry) GetVersion(ctx context.Context, namespace, name, version string) (*metadata.SkillVersion, error) {
	// Handle "latest" alias
	if version == "latest" {
		skill, err := r.repo.GetSkill(ctx, namespace, name)
		if err != nil {
			return nil, err
		}
		if skill == nil {
			return nil, fmt.Errorf("skill not found")
		}
		version = skill.LatestVersion
	}

	skillVersion, err := r.repo.GetVersion(ctx, namespace, name, version)
	if err != nil {
		return nil, err
	}
	if skillVersion == nil {
		return nil, fmt.Errorf("version not found")
	}

	return skillVersion, nil
}

// Download retrieves package data for a version
func (r *Registry) Download(ctx context.Context, namespace, name, version string) ([]byte, *metadata.SkillVersion, error) {
	skillVersion, err := r.GetVersion(ctx, namespace, name, version)
	if err != nil {
		return nil, nil, err
	}

	data, err := r.storage.Retrieve(skillVersion.DigestSHA256)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve package: %w", err)
	}

	return data, skillVersion, nil
}

// ListSkills lists skills with filters
func (r *Registry) ListSkills(ctx context.Context, filters map[string]interface{}, limit, offset int) (*metadata.SkillList, error) {
	return r.repo.ListSkills(ctx, filters, limit, offset)
}

// DeleteVersion soft-deletes a version
func (r *Registry) DeleteVersion(ctx context.Context, namespace, name, version string, actor string) error {
	if err := r.repo.DeleteVersion(ctx, namespace, name, version); err != nil {
		return err
	}

	// Log audit entry
	auditLog := &metadata.AuditLog{
		Action:    "delete",
		Namespace: namespace,
		Name:      name,
		Version:   version,
		Actor:     actor,
		Success:   true,
	}
	r.repo.LogAudit(ctx, auditLog)

	r.logger.Info("skill version deleted",
		"namespace", namespace,
		"name", name,
		"version", version,
		"actor", actor,
	)

	return nil
}

// Validate validates a package without publishing
func (r *Registry) Validate(ctx context.Context, data []byte, packageType string) (*validation.ValidationResult, error) {
	return r.validator.ValidatePackage(data, packageType)
}
