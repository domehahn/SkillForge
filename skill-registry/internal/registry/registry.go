package registry

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
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
	Force            bool
	CreatedBy        string
	Source           string
	OverrideMetadata bool
	SignatureDigest  string
	Provenance       *metadata.Provenance
}

// Publish publishes a skill package
func (r *Registry) Publish(ctx context.Context, namespace, name, version string, data []byte, packageType string, opts PublishOptions) (*metadata.SkillVersion, error) {
	// Validate name and version
	if err := validation.ValidateSkillName(name); err != nil {
		return nil, err
	}
	if err := validation.ValidateNamespace(namespace); err != nil {
		return nil, err
	}
	if err := validation.ValidateVersion(version); err != nil {
		return nil, err
	}

	// Validate package using publish-grade rules. Production publish does not accept SKILL.md-only packages.
	result, err := r.validator.ValidatePackageWithProfile(data, packageType, validation.ProfilePublish)
	if err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	if !result.Valid {
		return nil, fmt.Errorf("package validation failed: %s", strings.Join(result.Errors, "; "))
	}
	if result.Metadata == nil {
		return nil, fmt.Errorf("package validation failed: skill.yaml metadata is required")
	}
	if result.Metadata.Name != name {
		return nil, fmt.Errorf("package validation failed: manifest name %q does not match published name %q", result.Metadata.Name, name)
	}
	if result.Metadata.Namespace != "" && result.Metadata.Namespace != namespace {
		return nil, fmt.Errorf("package validation failed: manifest namespace %q does not match published namespace %q", result.Metadata.Namespace, namespace)
	}
	if result.Metadata.Version != version {
		return nil, fmt.Errorf("package validation failed: manifest version %q does not match published version %q", result.Metadata.Version, version)
	}

	// Check if version already exists
	existing, err := r.repo.GetVersion(ctx, namespace, name, version)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing version: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("version %s already exists; published versions are immutable", version)
	}

	// Store package
	digest, err := r.storage.Store(namespace, name, version, data)
	if err != nil {
		return nil, fmt.Errorf("failed to store package: %w", err)
	}

	latest, err := r.calculateLatest(ctx, namespace, name, version)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate latest: %w", err)
	}

	// Create or update skill
	skill := &metadata.Skill{
		Name:          name,
		Namespace:     namespace,
		LatestVersion: latest,
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
		SignatureDigest:  opts.SignatureDigest,
		Provenance:       opts.Provenance,
		Source:           "local",
	}
	if opts.SignatureDigest != "" {
		skillVersion.SignatureStatus = "signed"
	}

	if opts.Source != "" {
		skillVersion.Source = opts.Source
	}

	if result.Metadata != nil {
		skillVersion.EntrypointPath = result.Metadata.Entrypoint
		skillVersion.AgentCompatibility = compatibleMap(result.Metadata)
	}

	if err := r.repo.CreateVersion(ctx, skillVersion); err != nil {
		return nil, fmt.Errorf("failed to create version: %w", err)
	}
	if err := r.repo.SetDistTag(ctx, namespace, name, "latest", latest, opts.CreatedBy); err != nil {
		return nil, fmt.Errorf("failed to update latest dist-tag: %w", err)
	}
	if err := r.mirrorSkillArtifact(ctx, skill, skillVersion, opts.CreatedBy); err != nil {
		return nil, fmt.Errorf("failed to mirror skill artifact: %w", err)
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

func compatibleMap(manifest *metadata.SkillManifest) map[string]string {
	out := map[string]string{}
	for _, platform := range manifest.CompatibleWith {
		out[platform] = "supported"
	}
	for k, v := range manifest.Compatibility {
		out[k] = v
	}
	return out
}

func (r *Registry) calculateLatest(ctx context.Context, namespace, name, candidate string) (string, error) {
	skill, err := r.repo.GetSkill(ctx, namespace, name)
	if err != nil || skill == nil {
		return candidate, err
	}
	versions, err := r.repo.ListVersions(ctx, skill.ID)
	if err != nil {
		return "", err
	}
	all := append([]string{candidate}, versionsToStrings(versions)...)
	sort.SliceStable(all, func(i, j int) bool { return compareSemver(all[i], all[j]) > 0 })
	return all[0], nil
}

func versionsToStrings(versions []metadata.SkillVersion) []string {
	out := make([]string, 0, len(versions))
	for _, v := range versions {
		if !v.Yanked {
			out = append(out, v.Version)
		}
	}
	return out
}

func (r *Registry) mirrorSkillArtifact(ctx context.Context, skill *metadata.Skill, version *metadata.SkillVersion, actor string) error {
	artifact := &metadata.Artifact{
		Kind: metadata.ArtifactKindSkill, Namespace: skill.Namespace, Name: skill.Name,
		Description: skill.Description, LatestVersion: skill.LatestVersion, Visibility: skill.Visibility,
		Tags: skill.Tags, Owners: skill.Owners,
	}
	if err := r.repo.CreateArtifact(ctx, artifact); err != nil {
		return err
	}
	if existing, err := r.repo.GetArtifactVersion(ctx, artifact.Kind, artifact.Namespace, artifact.Name, version.Version); err != nil {
		return err
	} else if existing == nil {
		manifest := &metadata.ArtifactManifest{
			APIVersion: "skillforge.dev/v1", Kind: metadata.ArtifactKindSkill,
			Metadata: metadata.ArtifactMetadata{
				Namespace: skill.Namespace, Name: skill.Name, Version: version.Version,
				Description: skill.Description, Visibility: skill.Visibility, Tags: skill.Tags, Owners: skill.Owners,
			},
			Spec: metadata.ArtifactSpec{Entrypoint: version.EntrypointPath},
		}
		artifactVersion := &metadata.ArtifactVersion{
			ArtifactID: artifact.ID, Kind: artifact.Kind, Namespace: artifact.Namespace, Name: artifact.Name,
			Version: version.Version, DigestSHA256: version.DigestSHA256, PackageType: version.PackageType,
			SizeBytes: version.SizeBytes, Entrypoint: version.EntrypointPath, Manifest: manifest,
			Lockfile: &metadata.ArtifactLockfile{APIVersion: "skillforge.dev/v1", Root: "skill/" + skill.Namespace + "/" + skill.Name + "@" + version.Version, CreatedAt: time.Now()},
			OCIDescriptor: &metadata.OCIDescriptor{
				MediaType: "application/vnd.skillforge.artifact.manifest.v1+json", Digest: "sha256:" + version.DigestSHA256,
				Size: version.SizeBytes, ArtifactType: "application/vnd.skillforge.skill.v1",
			},
			SignatureStatus: version.SignatureStatus, ScanStatus: "pending", ValidationStatus: version.ValidationStatus,
			Source: version.Source, CreatedBy: actor,
		}
		if err := r.repo.CreateArtifactVersion(ctx, artifactVersion); err != nil {
			return err
		}
	}
	return r.repo.SetArtifactDistTag(ctx, artifact.ID, "latest", version.Version, actor)
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

	skill.Downloads, _ = r.repo.DownloadCount(ctx, namespace, name, "")
	for i := range versions {
		versions[i].Downloads, _ = r.repo.DownloadCount(ctx, namespace, name, versions[i].Version)
	}
	return skill, versions, nil
}

// GetVersion retrieves a specific version
func (r *Registry) GetVersion(ctx context.Context, namespace, name, version string) (*metadata.SkillVersion, error) {
	skillVersion, err := r.repo.GetVersion(ctx, namespace, name, version)
	if err != nil {
		return nil, err
	}
	if skillVersion == nil {
		tags, tagErr := r.repo.ListDistTags(ctx, namespace, name)
		if tagErr != nil {
			return nil, tagErr
		}
		if taggedVersion, ok := tags[version]; ok {
			skillVersion, err = r.repo.GetVersion(ctx, namespace, name, taggedVersion)
			if err != nil {
				return nil, err
			}
		}
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

	if err := r.repo.IncrementDownload(ctx, namespace, name, skillVersion.Version); err != nil {
		r.logger.Warn("failed to record download", "error", err)
	}
	return data, skillVersion, nil
}

// ListSkills lists skills with filters
func (r *Registry) ListSkills(ctx context.Context, filters map[string]interface{}, limit, offset int) (*metadata.SkillList, error) {
	list, err := r.repo.ListSkills(ctx, filters, limit, offset)
	if err != nil {
		return nil, err
	}
	for i := range list.Skills {
		list.Skills[i].Downloads, _ = r.repo.DownloadCount(ctx, list.Skills[i].Namespace, list.Skills[i].Name, "")
	}
	return list, nil
}

func (r *Registry) ListDistTags(ctx context.Context, namespace, name string) (map[string]string, error) {
	return r.repo.ListDistTags(ctx, namespace, name)
}

func (r *Registry) SetDistTag(ctx context.Context, namespace, name, tag, version, actor string) error {
	if err := validation.ValidateNamespace(namespace); err != nil {
		return err
	}
	if err := validation.ValidateSkillName(name); err != nil {
		return err
	}
	if err := validation.ValidateDistTag(tag); err != nil {
		return err
	}
	if err := validation.ValidateVersion(version); err != nil {
		return err
	}
	if err := r.repo.SetDistTag(ctx, namespace, name, tag, version, actor); err != nil {
		return err
	}
	return r.repo.LogAudit(ctx, &metadata.AuditLog{
		Action: "dist-tag", Namespace: namespace, Name: name, Version: version,
		Actor: actor, Success: true, Message: tag,
	})
}

func (r *Registry) DeprecateVersion(ctx context.Context, namespace, name, version, reason, actor string) error {
	if reason == "" {
		return fmt.Errorf("reason is required")
	}
	if _, err := r.GetVersion(ctx, namespace, name, version); err != nil {
		return err
	}
	if err := r.repo.DeprecateVersion(ctx, namespace, name, version, reason, actor); err != nil {
		return err
	}
	return r.repo.LogAudit(ctx, &metadata.AuditLog{
		Action: "deprecate", Namespace: namespace, Name: name, Version: version,
		Actor: actor, Success: true, Message: reason,
	})
}

func (r *Registry) YankVersion(ctx context.Context, namespace, name, version, reason, actor string) error {
	if reason == "" {
		return fmt.Errorf("reason is required")
	}
	if _, err := r.GetVersion(ctx, namespace, name, version); err != nil {
		return err
	}
	if err := r.repo.YankVersion(ctx, namespace, name, version, reason, actor); err != nil {
		return err
	}
	return r.repo.LogAudit(ctx, &metadata.AuditLog{
		Action: "yank", Namespace: namespace, Name: name, Version: version,
		Actor: actor, Success: true, Message: reason,
	})
}

func (r *Registry) UnyankVersion(ctx context.Context, namespace, name, version, actor string) error {
	if _, err := r.GetVersion(ctx, namespace, name, version); err != nil {
		return err
	}
	if err := r.repo.UnyankVersion(ctx, namespace, name, version, actor); err != nil {
		return err
	}
	return r.repo.LogAudit(ctx, &metadata.AuditLog{
		Action: "unyank", Namespace: namespace, Name: name, Version: version,
		Actor: actor, Success: true,
	})
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
