package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/domehahn/sklib/registryapi"
	"github.com/domehahn/sklib/spec"
	"github.com/go-chi/chi/v5"
	"github.com/skillforge/skill-registry/internal/audit"
	"github.com/skillforge/skill-registry/internal/auth"
	"github.com/skillforge/skill-registry/internal/config"
	"github.com/skillforge/skill-registry/internal/email"
	"github.com/skillforge/skill-registry/internal/metadata"
	"github.com/skillforge/skill-registry/internal/observability"
	"github.com/skillforge/skill-registry/internal/registry"
	"github.com/skillforge/skill-registry/internal/validation"
)

// Handler handles HTTP requests
type Handler struct {
	registry *registry.Registry
	auth     *auth.Authenticator
	audit    *audit.Repository // may be nil; no audit logging when nil
	logger   *slog.Logger
	config   *config.Config
	email    *email.Sender
}

// NewHandler creates a new API handler. auditRepo may be nil to disable audit logging.
func NewHandler(reg *registry.Registry, authenticator *auth.Authenticator, auditRepo *audit.Repository, logger *slog.Logger, cfg *config.Config) *Handler {
	return &Handler{
		registry: reg,
		auth:     authenticator,
		audit:    auditRepo,
		logger:   logger,
		config:   cfg,
		email:    email.NewSender(cfg.Email),
	}
}

func (h *Handler) logAudit(r *http.Request, actor, action, target string) {
	if h.audit == nil {
		return
	}
	_ = h.audit.Log(r.Context(), audit.Entry{
		Actor:     actor,
		Action:    action,
		Target:    target,
		IPAddress: clientIP(r),
	})
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.SplitN(xff, ",", 2)[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// RegisterRoutes registers all API routes
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Health endpoints
	r.Get("/healthz", h.HealthCheck)
	r.Get("/readyz", h.ReadyCheck)

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		maxBytes := int64(h.config.Storage.MaxPackageSizeMB)*1024*1024 + 65536 // +64 KB for headers/manifest
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
				next.ServeHTTP(w, r)
			})
		})

		r.Get("/metadata", h.GetMetadata)
		r.Get("/capabilities", h.GetCapabilities)
		r.Get("/stats", h.GetStats)
		r.Get("/artifacts/facets", h.GetFacets)
		r.Get("/namespaces/{namespace}", h.GetNamespace)
		r.Get("/namespaces/{namespace}/feed", h.AtomFeed)

		// Auth endpoints (no authentication required)
		r.Post("/auth/login", h.Login)
		r.Get("/auth/verify-email", h.VerifyEmail)
		r.Post("/auth/verify-email", h.VerifyEmail)

		// Public read endpoints (no auth required)
		r.Group(func(r chi.Router) {
			r.Use(h.auth.Middleware())
			r.Get("/artifacts", h.ListArtifacts)
			r.Get("/artifacts/{kind}/{namespace}/{name}", h.GetArtifact)
			r.Get("/artifacts/{kind}/{namespace}/{name}/versions/{version}", h.GetArtifactVersion)
			r.Get("/artifacts/{kind}/{namespace}/{name}/versions/{version}/download", h.DownloadArtifact)
			r.Get("/artifacts/{kind}/{namespace}/{name}/versions/{version}/graph", h.ArtifactGraph)
			r.Get("/artifacts/{kind}/{namespace}/{name}/versions/{version}/lockfile", h.ArtifactLockfile)
			r.Get("/artifacts/{kind}/{namespace}/{name}/versions/{version}/attestations", h.ListArtifactAttestations)
			r.Get("/artifacts/{kind}/{namespace}/{name}/promotions", h.ListPromotions)
			r.Get("/skills", h.ListSkills)
			r.Get("/skills/{namespace}/{name}", h.GetSkill)
			r.Get("/skills/{namespace}/{name}/resolve", h.ResolveSkill)
			r.Get("/skills/{namespace}/{name}/dist-tags", h.ListDistTags)
			r.Get("/skills/{namespace}/{name}/versions/{version}", h.GetVersion)
			r.Get("/skills/{namespace}/{name}/versions/{version}/download", h.Download)
		})

		// Token management — scoped individually so each operation requires its own scope.
		r.With(h.auth.Middleware("token:create")).Post("/tokens", h.CreateToken)
		r.With(h.auth.Middleware("read")).Get("/tokens", h.ListTokens)
		r.With(h.auth.Middleware("token:revoke")).Delete("/tokens/{id}", h.RevokeToken)

		// Account self-service
		r.With(h.auth.Middleware("write")).Patch("/account/password", h.ChangePassword)
		r.With(h.auth.Middleware("write")).Patch("/account/profile", h.UpdateProfile)

		// Notifications
		r.With(h.auth.Middleware("read")).Get("/notifications", h.GetNotifications)
		r.With(h.auth.Middleware("read")).Put("/notifications/read", h.MarkNotificationsRead)
		r.With(h.auth.Middleware("read")).Delete("/notifications/{id}", h.DeleteNotification)

		// Public artifact stats and dependents
		r.Get("/artifacts/{kind}/{namespace}/{name}/stats", h.GetArtifactStats)
		r.Get("/artifacts/mcp/{namespace}/{name}/mcp.json", h.GetMCPManifest)
		r.Get("/artifacts/{namespace}/{name}/dependents", h.GetArtifactDependents)
		r.Get("/namespaces/{namespace}/pinned", h.GetPinned)
		r.Get("/publishers/top", h.ListTopPublishers)

		// Stars (public read, auth write)
		r.With(h.auth.Middleware()).Get("/artifacts/{kind}/{namespace}/{name}/stars", h.GetArtifactStarInfo)
		r.With(h.auth.Middleware("write")).Post("/artifacts/{kind}/{namespace}/{name}/star", h.StarArtifact)
		r.With(h.auth.Middleware("write")).Delete("/artifacts/{kind}/{namespace}/{name}/star", h.UnstarArtifact)

		// Scan results (public read)
		r.Get("/artifacts/{kind}/{namespace}/{name}/versions/{version}/scan", h.ListScanResults)

		// Comments
		r.Get("/artifacts/{kind}/{namespace}/{name}/comments", h.ListComments)
		r.With(h.auth.Middleware("write")).Post("/artifacts/{kind}/{namespace}/{name}/comments", h.AddComment)
		r.With(h.auth.Middleware("write")).Patch("/comments/{id}", h.UpdateComment)
		r.With(h.auth.Middleware("write")).Delete("/comments/{id}", h.DeleteComment)

		// Collections
		r.Get("/namespaces/{owner}/collections", h.ListCollections)
		r.Get("/namespaces/{owner}/collections/{slug}", h.GetCollection)
		r.With(h.auth.Middleware("write")).Post("/collections", h.CreateCollection)
		r.With(h.auth.Middleware("write")).Put("/namespaces/{owner}/collections/{slug}", h.UpdateCollection)
		r.With(h.auth.Middleware("write")).Delete("/namespaces/{owner}/collections/{slug}", h.DeleteCollection)

		// Namespace insights
		r.With(h.auth.Middleware()).Get("/namespaces/{namespace}/insights", h.GetNamespaceInsights)

		// Artifact transfer
		r.With(h.auth.Middleware("write")).Post("/artifacts/{kind}/{namespace}/{name}/transfer", h.TransferArtifact)

		// Follows
		r.Get("/namespaces/{namespace}/follow", h.GetFollowInfo)
		r.With(h.auth.Middleware()).Post("/namespaces/{namespace}/follow", h.FollowNamespace)
		r.With(h.auth.Middleware()).Delete("/namespaces/{namespace}/follow", h.UnfollowNamespace)
		r.With(h.auth.Middleware()).Get("/account/following", h.GetFollowing)

		// Write endpoints (require write scope)
		r.Group(func(r chi.Router) {
			r.Use(h.auth.Middleware("write"))
			r.Put("/skills/{namespace}/{name}/versions/{version}", h.Publish)
			r.Post("/validate", h.Validate)
			r.Put("/skills/{namespace}/{name}/dist-tags/{tag}", h.SetDistTag)
			r.Post("/skills/{namespace}/{name}/versions/{version}/deprecate", h.DeprecateVersion)
			r.Post("/skills/{namespace}/{name}/versions/{version}/yank", h.YankVersion)
			r.Post("/skills/{namespace}/{name}/versions/{version}/unyank", h.UnyankVersion)
			r.Put("/artifacts/{kind}/{namespace}/{name}/versions/{version}", h.PublishArtifact)
			r.Patch("/artifacts/{kind}/{namespace}/{name}", h.PatchArtifact)
			r.Patch("/artifacts/{kind}/{namespace}/{name}/versions/{version}", h.PatchArtifactVersion)
			r.Patch("/namespaces/{namespace}", h.PatchNamespace)
			r.Post("/artifacts/{kind}/{namespace}/{name}/promotions", h.PromoteArtifact)
			r.Put("/artifacts/{kind}/{namespace}/{name}/versions/{version}/attestations", h.AttestArtifact)
			r.Post("/artifacts/{kind}/{namespace}/{name}/versions/{version}/attestations", h.CreateArtifactAttestation)
			r.Get("/namespaces/{namespace}/webhooks", h.ListWebhooks)
			r.Post("/namespaces/{namespace}/webhooks", h.CreateWebhook)
			r.Delete("/namespaces/{namespace}/webhooks/{id}", h.DeleteWebhook)
			r.Post("/namespaces/{namespace}/webhooks/{id}/test", h.TestWebhook)
			r.Get("/namespaces/{namespace}/webhooks/{id}/deliveries", h.ListWebhookDeliveries)
			r.Put("/namespaces/{namespace}/pinned", h.SetPinned)
		})

		// Namespace admin endpoints (require namespace:admin scope)
		r.Group(func(r chi.Router) {
			r.Use(h.auth.Middleware("namespace:admin"))
			r.Get("/namespaces/{namespace}/members", h.ListNamespaceMembers)
			r.Put("/namespaces/{namespace}/members", h.UpsertNamespaceMember)
			r.Delete("/namespaces/{namespace}/members/{username}", h.RemoveNamespaceMember)
		})

		// Delete endpoints (require delete scope)
		r.Group(func(r chi.Router) {
			r.Use(h.auth.Middleware("delete"))
			r.Delete("/skills/{namespace}/{name}/versions/{version}", h.DeleteVersion)
		})

		// Admin-only endpoints
		r.Group(func(r chi.Router) {
			r.Use(h.auth.Middleware("admin"))
			r.Get("/audit", h.GetAuditLog)
			r.Get("/admin/users", h.AdminListUsers)
			r.Post("/admin/users", h.AdminCreateUser)
			r.Post("/admin/users/{username}/send-verification", h.AdminSendVerification)
			r.Put("/admin/users/{username}/role", h.AdminUpdateUserRole)
			r.Delete("/admin/users/{username}", h.AdminDeleteUser)
			r.Get("/admin/namespaces", h.AdminListNamespaces)
			r.Put("/admin/namespaces/{namespace}/verify", h.AdminVerifyNamespace)
		})

		// Metrics — public so infra scrapers work without a token
		r.Get("/metrics", h.GetMetrics)

		// OpenAPI spec
		r.Get("/openapi.yaml", h.GetOpenAPISpec)
	})

	// API docs (Redoc)
	r.Get("/api-docs", h.GetAPIDocs)

	// Serve static files from web UI (if exists)
	h.ServeStaticFiles(r)
}

// HealthCheck handles health check requests
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ReadyCheck handles readiness check requests
func (h *Handler) ReadyCheck(w http.ResponseWriter, r *http.Request) {
	if err := h.registry.Ping(r.Context()); err != nil {
		h.logger.Error("readyz: db ping failed", "error", err)
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not ready", "error": "database unavailable"})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// GetMetadata returns registry metadata
func (h *Handler) GetMetadata(w http.ResponseWriter, r *http.Request) {
	authMode := "disabled"
	if h.config.Auth.Enabled {
		authMode = "token"
	}

	metadata := RegistryMetadata{
		Name:                  "Skill Registry",
		Version:               "1.0.0",
		SupportedPackageTypes: h.config.Storage.AllowedPackageTypes,
		AuthMode:              authMode,
		Capabilities: []string{
			"artifacts", "skills", "agents", "flows", "prompts", "tools", "bundles",
			"dependencies", "lockfiles", "graphs", "promotions", "namespace-acls",
			"webhooks", "attestations", "oci-descriptors", "immutable-versions",
		},
	}

	WriteJSON(w, http.StatusOK, metadata)
}

func (h *Handler) GetCapabilities(w http.ResponseWriter, r *http.Request) {
	authMode := "disabled"
	if h.config.Auth.Enabled {
		authMode = "token"
	}
	caps := registryapi.FullCapabilities("v1")
	caps.PackageTypes = h.config.Storage.AllowedPackageTypes
	caps.MaxPackageSizeBytes = int64(h.config.Storage.MaxPackageSizeMB) * 1024 * 1024
	// Registry-specific extensions beyond the shared contract.
	type capabilitiesExt struct {
		registryapi.CapabilitiesResponse
		AuthMode           string   `json:"auth_mode"`
		AuthEnabled        bool     `json:"auth_enabled"`
		SupportedPlatforms []string `json:"supported_platforms"`
	}
	WriteJSON(w, http.StatusOK, capabilitiesExt{
		CapabilitiesResponse: caps,
		AuthMode:             authMode,
		AuthEnabled:          h.config.Auth.Enabled,
		SupportedPlatforms:   validation.SupportedPlatforms(),
	})
}

// Publish handles skill publishing
func (h *Handler) Publish(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	version := chi.URLParam(r, "version")

	// Parse query parameters
	force := r.URL.Query().Get("force") == "true"

	// Determine package type from Content-Type
	contentType := r.Header.Get("Content-Type")
	packageType := ""
	switch contentType {
	case "application/gzip", "application/x-gzip":
		packageType = "tgz"
	case "application/zip":
		packageType = "zip"
	default:
		WriteError(w, http.StatusBadRequest, "INVALID_CONTENT_TYPE", "Content-Type must be application/gzip or application/zip")
		return
	}

	// Enforce namespace membership — open when no members are configured.
	if !h.requireNamespaceRole(w, r, namespace, "maintainer") {
		return
	}

	// Read package data
	data, err := io.ReadAll(io.LimitReader(r.Body, int64(h.config.Storage.MaxPackageSizeMB)*1024*1024+1))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "READ_ERROR", "Failed to read request body")
		return
	}

	// Publish
	actor := auth.ActorFromContext(ctx)
	skillVersion, err := h.registry.Publish(ctx, namespace, name, version, data, packageType, registry.PublishOptions{
		Force:           force,
		CreatedBy:       actor,
		Source:          "local",
		SignatureDigest: r.Header.Get("X-SkillForge-Signature"),
	})
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			WriteError(w, http.StatusConflict, "VERSION_EXISTS", err.Error())
		} else if strings.Contains(err.Error(), "validation failed") {
			WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
		} else {
			h.logger.Error("publish failed", "error", err)
			WriteError(w, http.StatusInternalServerError, "PUBLISH_FAILED", "Failed to publish skill")
		}
		return
	}

	h.logAudit(r, actor, "skill.publish", namespace+"/"+name+"@"+version)
	WriteJSON(w, http.StatusCreated, registryapi.PublishResponse{
		Namespace:   namespace,
		Name:        name,
		Version:     version,
		DownloadURL: h.downloadURL(r, namespace, name, version),
		SHA256:      skillVersion.DigestSHA256,
		Created:     true,
	})
}

// ListSkills handles skill listing
func (h *Handler) ListSkills(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	filters := make(map[string]interface{})
	if q := r.URL.Query().Get("q"); q != "" {
		filters["q"] = q
	}
	if namespace := r.URL.Query().Get("namespace"); namespace != "" {
		filters["namespace"] = namespace
	}
	if tag := r.URL.Query().Get("tag"); tag != "" {
		filters["tag"] = tag
	}
	if deprecated := r.URL.Query().Get("deprecated"); deprecated == "true" {
		filters["deprecated"] = true
	} else if deprecated == "false" {
		filters["deprecated"] = false
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	skills, err := h.registry.ListSkills(ctx, filters, limit, offset)
	if err != nil {
		h.logger.Error("list skills failed", "error", err)
		WriteError(w, http.StatusInternalServerError, "LIST_FAILED", "Failed to list skills")
		return
	}

	results := make([]registryapi.SearchResult, 0, len(skills.Skills))
	for _, s := range skills.Skills {
		results = append(results, registryapi.SearchResult{
			Namespace:     s.Namespace,
			Name:          s.Name,
			LatestVersion: s.LatestVersion,
			Description:   s.Description,
			Tags:          s.Tags,
		})
	}
	WriteJSON(w, http.StatusOK, registryapi.SearchResponse{Results: results, Total: skills.Total})
}

func (h *Handler) ResolveSkill(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	constraint := r.URL.Query().Get("constraint")
	if constraint == "" {
		constraint = "*"
	}
	_, versions, err := h.registry.GetSkill(ctx, namespace, name)
	if err != nil {
		WriteError(w, http.StatusNotFound, "SKILL_NOT_FOUND", err.Error())
		return
	}
	resolved := resolveAPIVersion(versions, constraint)
	if resolved == nil {
		WriteError(w, http.StatusNotFound, "NO_MATCHING_VERSION", "no version satisfies the requested constraint")
		return
	}
	WriteJSON(w, http.StatusOK, registryapi.ResolveResponse{
		Namespace:      namespace,
		Name:           name,
		Version:        resolved.Version,
		DownloadURL:    h.downloadURL(r, namespace, name, resolved.Version),
		SHA256:         resolved.DigestSHA256,
		PackageType:    resolved.PackageType,
		CompatibleWith: compatibleWithPlatforms(resolved.AgentCompatibility),
	})
}

func (h *Handler) DeprecateVersion(w http.ResponseWriter, r *http.Request) {
	h.governVersion(w, r, "deprecate")
}

func (h *Handler) YankVersion(w http.ResponseWriter, r *http.Request) {
	h.governVersion(w, r, "yank")
}

func (h *Handler) UnyankVersion(w http.ResponseWriter, r *http.Request) {
	h.governVersion(w, r, "unyank")
}

func (h *Handler) governVersion(w http.ResponseWriter, r *http.Request, action string) {
	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	version := chi.URLParam(r, "version")
	actor := auth.ActorFromContext(r.Context())
	var err error
	switch action {
	case "deprecate":
		err = h.registry.DeprecateVersion(r.Context(), namespace, name, version, body.Reason, actor)
	case "yank":
		err = h.registry.YankVersion(r.Context(), namespace, name, version, body.Reason, actor)
	case "unyank":
		err = h.registry.UnyankVersion(r.Context(), namespace, name, version, actor)
	}
	if err != nil {
		WriteError(w, http.StatusBadRequest, "GOVERNANCE_FAILED", err.Error())
		return
	}
	h.logAudit(r, actor, "skill."+action, namespace+"/"+name+"@"+version)
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "action": action})
}

// GetSkill handles retrieving a skill with its versions
func (h *Handler) GetSkill(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")

	skill, versions, err := h.registry.GetSkill(ctx, namespace, name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "Skill not found")
		} else {
			h.logger.Error("get skill failed", "error", err)
			WriteError(w, http.StatusInternalServerError, "GET_FAILED", "Failed to get skill")
		}
		return
	}

	distTags, err := h.registry.ListDistTags(ctx, namespace, name)
	if err != nil {
		h.logger.Error("list dist-tags failed", "error", err)
		WriteError(w, http.StatusInternalServerError, "GET_FAILED", "Failed to get dist-tags")
		return
	}

	type skillDetailsResponse struct {
		registryapi.SkillInfoResponse
		DistTags map[string]string `json:"dist_tags,omitempty"`
	}
	WriteJSON(w, http.StatusOK, skillDetailsResponse{
		SkillInfoResponse: registryapi.SkillInfoResponse{
			Namespace:     skill.Namespace,
			Name:          skill.Name,
			LatestVersion: skill.LatestVersion,
			Description:   skill.Description,
			Tags:          skill.Tags,
			Owners:        skill.Owners,
			Versions:      toRegistryAPIVersions(versions),
		},
		DistTags: distTags,
	})
}

// GetVersion handles retrieving a specific version
func (h *Handler) GetVersion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	version := chi.URLParam(r, "version")

	skillVersion, err := h.registry.GetVersion(ctx, namespace, name, version)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "Version not found")
		} else {
			h.logger.Error("get version failed", "error", err)
			WriteError(w, http.StatusInternalServerError, "GET_FAILED", "Failed to get version")
		}
		return
	}

	WriteJSON(w, http.StatusOK, skillVersion)
}

// Download handles downloading a skill package
func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	version := chi.URLParam(r, "version")

	data, skillVersion, err := h.registry.Download(ctx, namespace, name, version)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "Package not found")
		} else {
			h.logger.Error("download failed", "error", err)
			WriteError(w, http.StatusInternalServerError, "DOWNLOAD_FAILED", "Failed to download package")
		}
		return
	}

	// Set headers
	contentType := "application/gzip"
	extension := "tgz"
	if skillVersion.PackageType == "zip" {
		contentType = "application/zip"
		extension = "zip"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Skill-Digest-SHA256", skillVersion.DigestSHA256)
	filename := fmt.Sprintf("%s-%s-%s.%s", namespace, name, skillVersion.Version, extension)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (h *Handler) ListDistTags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.registry.ListDistTags(r.Context(), chi.URLParam(r, "namespace"), chi.URLParam(r, "name"))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}
		WriteError(w, http.StatusInternalServerError, "GET_FAILED", "Failed to list dist-tags")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"dist_tags": tags})
}

func (h *Handler) SetDistTag(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Version string `json:"version"`
	}
	if err := ParseJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}
	err := h.registry.SetDistTag(
		r.Context(),
		chi.URLParam(r, "namespace"),
		chi.URLParam(r, "name"),
		chi.URLParam(r, "tag"),
		req.Version,
		auth.ActorFromContext(r.Context()),
	)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}
		WriteError(w, http.StatusBadRequest, "INVALID_DIST_TAG", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"tag": chi.URLParam(r, "tag"), "version": req.Version})
}

// DeleteVersion handles deleting a version
func (h *Handler) DeleteVersion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	version := chi.URLParam(r, "version")

	actor := auth.ActorFromContext(ctx)
	if err := h.registry.DeleteVersion(ctx, namespace, name, version, actor); err != nil {
		h.logger.Error("delete version failed", "error", err)
		WriteError(w, http.StatusInternalServerError, "DELETE_FAILED", "Failed to delete version")
		return
	}

	h.logAudit(r, actor, "skill.delete", namespace+"/"+name+"@"+version)
	WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// Validate handles package validation without publishing
func (h *Handler) Validate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Determine package type from Content-Type
	contentType := r.Header.Get("Content-Type")
	packageType := ""
	switch contentType {
	case "application/gzip", "application/x-gzip":
		packageType = "tgz"
	case "application/zip":
		packageType = "zip"
	default:
		WriteError(w, http.StatusBadRequest, "INVALID_CONTENT_TYPE", "Content-Type must be application/gzip or application/zip")
		return
	}

	// Read package data
	data, err := io.ReadAll(io.LimitReader(r.Body, int64(h.config.Storage.MaxPackageSizeMB)*1024*1024+1))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "READ_ERROR", "Failed to read request body")
		return
	}

	// Validate
	result, err := h.registry.Validate(ctx, data, packageType)
	if err != nil {
		h.logger.Error("validation error", "error", err)
		WriteError(w, http.StatusInternalServerError, "VALIDATION_ERROR", "Validation failed")
		return
	}

	status := http.StatusOK
	if !result.Valid {
		status = http.StatusBadRequest
	}

	WriteJSON(w, status, result)
}

// Login handles user authentication
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request body
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := ParseJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	// Authenticate user
	userRepo := h.auth.GetUserRepo()
	user, err := userRepo.AuthenticateUser(ctx, req.Username, req.Password)
	if err != nil {
		h.logger.Warn("authentication failed", "username", req.Username, "error", err)
		WriteError(w, http.StatusUnauthorized, "AUTH_FAILED", "Invalid username or password")
		return
	}
	if h.config.Auth.RequireEmailVerification && user.Role != "admin" && !user.EmailVerified {
		h.logger.Warn("authentication blocked for unverified email", "username", req.Username)
		WriteError(w, http.StatusForbidden, "EMAIL_NOT_VERIFIED", "Email verification is required before login")
		return
	}

	// Determine scopes based on role
	scopes := []string{"read", "write"}
	if user.Role == "admin" {
		scopes = []string{"read", "write", "delete", "admin", "token:create", "token:revoke", "namespace:admin"}
	}

	// Create session token (valid for 30 days)
	expiresIn := 30 * 24 * time.Hour
	token, err := userRepo.CreateToken(ctx, user.ID, "login-session", scopes, &expiresIn)
	if err != nil {
		h.logger.Error("failed to create token", "error", err)
		WriteError(w, http.StatusInternalServerError, "TOKEN_CREATION_FAILED", "Failed to create session token")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"token":          token.Token,
		"user":           user.Username,
		"role":           user.Role,
		"scopes":         scopes,
		"email":          user.Email,
		"email_verified": user.EmailVerified,
		"expires_at":     token.ExpiresAt,
	})
}

// VerifyEmail marks an account email as verified when given a valid token.
func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" && r.Method == http.MethodPost {
		var req struct {
			Token string `json:"token"`
		}
		if err := ParseJSON(r, &req); err != nil {
			WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
			return
		}
		token = req.Token
	}
	if token == "" {
		WriteError(w, http.StatusBadRequest, "MISSING_TOKEN", "Verification token is required")
		return
	}
	user, err := h.auth.GetUserRepo().VerifyEmailToken(r.Context(), token)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "VERIFY_EMAIL_FAILED", err.Error())
		return
	}
	h.logAudit(r, user.Username, "email_verify", user.Username)
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":         "verified",
		"user":           user.Username,
		"email":          user.Email,
		"email_verified": user.EmailVerified,
	})
}

// CreateToken handles API token creation
func (h *Handler) CreateToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get authenticated user
	user := auth.UserFromContext(ctx)
	if user == nil {
		WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Parse request
	var req struct {
		Name      string   `json:"name"`
		Scopes    []string `json:"scopes"`
		ExpiresIn *int64   `json:"expires_in,omitempty"` // seconds
	}

	if err := ParseJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	// Validate scopes — non-admin users may only create tokens narrower than their own role.
	allowedScopes := []string{"read", "write"}
	if user.Role == "admin" {
		allowedScopes = []string{"read", "write", "delete", "admin", "token:create", "token:revoke", "namespace:admin"}
	}

	for _, scope := range req.Scopes {
		if !contains(allowedScopes, scope) {
			WriteError(w, http.StatusForbidden, "INVALID_SCOPE", fmt.Sprintf("Scope '%s' not allowed for your role", scope))
			return
		}
	}

	// Calculate expiration
	var expiresIn *time.Duration
	if req.ExpiresIn != nil {
		duration := time.Duration(*req.ExpiresIn) * time.Second
		expiresIn = &duration
	}

	// Create token
	userRepo := h.auth.GetUserRepo()
	token, err := userRepo.CreateToken(ctx, user.ID, req.Name, req.Scopes, expiresIn)
	if err != nil {
		h.logger.Error("failed to create token", "error", err)
		WriteError(w, http.StatusInternalServerError, "TOKEN_CREATION_FAILED", "Failed to create token")
		return
	}

	h.logAudit(r, user.Username, "token.create", req.Name)
	WriteJSON(w, http.StatusCreated, token)
}

// ListTokens handles listing user's tokens
func (h *Handler) ListTokens(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get authenticated user
	user := auth.UserFromContext(ctx)
	if user == nil {
		WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	// List tokens
	userRepo := h.auth.GetUserRepo()
	tokens, err := userRepo.ListTokens(ctx, user.ID)
	if err != nil {
		h.logger.Error("failed to list tokens", "error", err)
		WriteError(w, http.StatusInternalServerError, "LIST_FAILED", "Failed to list tokens")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"tokens": tokens,
	})
}

// RevokeToken handles token revocation
func (h *Handler) RevokeToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get authenticated user
	user := auth.UserFromContext(ctx)
	if user == nil {
		WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Parse token ID
	tokenIDStr := chi.URLParam(r, "id")
	tokenID, err := strconv.ParseInt(tokenIDStr, 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_TOKEN_ID", "Invalid token ID")
		return
	}

	// Revoke token
	userRepo := h.auth.GetUserRepo()
	if err := userRepo.RevokeToken(ctx, tokenID, user.ID); err != nil {
		h.logger.Error("failed to revoke token", "error", err)
		WriteError(w, http.StatusInternalServerError, "REVOKE_FAILED", "Failed to revoke token")
		return
	}

	h.logAudit(r, user.Username, "token.revoke", tokenIDStr)
	WriteJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// GetMetrics returns registry-wide counters in Prometheus exposition format.
func (h *Handler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	snap, err := h.registry.Metrics(r.Context())
	if err != nil {
		h.logger.Error("metrics query failed", "error", err)
		WriteError(w, http.StatusInternalServerError, "METRICS_FAILED", "Failed to query metrics")
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	observability.DefaultMetrics().WritePrometheus(w)
	writeGauge := func(name, help string, value int64) {
		_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n%s %d\n", name, help, name, name, value)
	}
	writeGauge("skillforge_total_skills", "Total registered legacy skill records.", snap.TotalSkills)
	writeGauge("skillforge_total_versions", "Total registered legacy skill versions.", snap.TotalVersions)
	writeGauge("skillforge_active_versions", "Legacy skill versions that are neither yanked nor deprecated.", snap.ActiveVersions)
	writeGauge("skillforge_yanked_versions", "Legacy skill versions marked as yanked.", snap.YankedVersions)
	writeGauge("skillforge_deprecated_versions", "Legacy skill versions marked as deprecated.", snap.DeprecatedVersions)
	writeGauge("skillforge_total_downloads", "Total legacy skill downloads recorded.", snap.TotalDownloads)
	writeGauge("skillforge_active_tokens", "API tokens that are not revoked.", snap.ActiveTokens)
	writeGauge("skillforge_total_artifacts", "Total registered artifacts.", snap.TotalArtifacts)
	writeGauge("skillforge_total_namespaces", "Total namespaces with artifacts.", snap.TotalNamespaces)
}

// GetAuditLog returns recent audit log entries. Requires admin scope.
func (h *Handler) GetAuditLog(w http.ResponseWriter, r *http.Request) {
	if h.audit == nil {
		WriteJSON(w, http.StatusOK, map[string]interface{}{"entries": []struct{}{}})
		return
	}
	actor := r.URL.Query().Get("actor")
	action := r.URL.Query().Get("action")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	entries, err := h.audit.List(r.Context(), actor, action, limit)
	if err != nil {
		h.logger.Error("audit log query failed", "error", err)
		WriteError(w, http.StatusInternalServerError, "AUDIT_FAILED", "Failed to query audit log")
		return
	}
	if entries == nil {
		entries = []audit.Entry{}
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"entries": entries})
}

// ServeStaticFiles serves the web UI if available
func (h *Handler) ServeStaticFiles(r chi.Router) {
	// Check if web/dist directory exists
	webDir := "./web/dist"
	if _, err := os.Stat(webDir); os.IsNotExist(err) {
		h.logger.Info("web UI not found, skipping static file serving", "path", webDir)
		return
	}

	h.logger.Info("serving web UI", "path", webDir)

	// Serve static files
	fileServer := http.FileServer(http.Dir(webDir))

	r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
		// If file exists, serve it
		path := filepath.Join(webDir, req.URL.Path)
		if _, err := os.Stat(path); err == nil {
			// File exists, serve it
			http.StripPrefix("", fileServer).ServeHTTP(w, req)
			return
		}

		// For client-side routing, serve index.html
		indexPath := filepath.Join(webDir, "index.html")
		http.ServeFile(w, req, indexPath)
	})
}

// Helper function to check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func resolveAPIVersion(versions []metadata.SkillVersion, constraint string) *metadata.SkillVersion {
	var candidates []metadata.SkillVersion
	var deprecated []metadata.SkillVersion
	for _, version := range versions {
		if version.Yanked {
			continue
		}
		if !(constraint == "" || constraint == "*" || constraint == "latest" || version.Version == constraint || apiCaretAllows(constraint, version.Version)) {
			continue
		}
		if version.Deprecated {
			deprecated = append(deprecated, version)
		} else {
			candidates = append(candidates, version)
		}
	}
	if len(candidates) == 0 {
		candidates = deprecated
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool { return apiCompareVersion(candidates[i].Version, candidates[j].Version) > 0 })
	return &candidates[0]
}

func apiCaretAllows(constraint, version string) bool {
	if !strings.HasPrefix(constraint, "^") {
		return false
	}
	base := strings.TrimPrefix(constraint, "^")
	bp, vp := apiVersionParts(base), apiVersionParts(version)
	return vp[0] == bp[0] && apiCompareVersion(version, base) >= 0
}

func apiCompareVersion(a, b string) int {
	ap, bp := apiVersionParts(a), apiVersionParts(b)
	for i := 0; i < 3; i++ {
		if ap[i] > bp[i] {
			return 1
		}
		if ap[i] < bp[i] {
			return -1
		}
	}
	// Same numeric parts — pre-release has lower precedence than release (semver §9).
	aPre, bPre := apiPreRelease(a), apiPreRelease(b)
	switch {
	case aPre == "" && bPre == "":
		return 0
	case aPre == "" && bPre != "":
		return 1 // release > pre-release
	case aPre != "" && bPre == "":
		return -1 // pre-release < release
	default:
		return strings.Compare(aPre, bPre)
	}
}

func apiPreRelease(v string) string {
	base := strings.Split(v, "+")[0]
	parts := strings.SplitN(base, "-", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

func apiVersionParts(v string) [3]int {
	base := strings.Split(strings.Split(v, "+")[0], "-")[0]
	parts := strings.Split(base, ".")
	var out [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		n, _ := strconv.Atoi(parts[i])
		out[i] = n
	}
	return out
}

func (h *Handler) downloadURL(r *http.Request, namespace, name, version string) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/api/v1/skills/%s/%s/versions/%s/download", scheme, r.Host, namespace, name, version)
}

func compatibleWithPlatforms(agentCompat map[string]string) []spec.Platform {
	out := make([]spec.Platform, 0, len(agentCompat))
	for platform := range agentCompat {
		out = append(out, spec.Platform(platform))
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func toRegistryAPIVersions(versions []metadata.SkillVersion) []registryapi.SkillVersion {
	out := make([]registryapi.SkillVersion, 0, len(versions))
	for _, v := range versions {
		rv := registryapi.SkillVersion{
			Version:           v.Version,
			Deprecated:        v.Deprecated,
			DeprecationReason: v.DeprecationReason,
			Yanked:            v.Yanked,
			YankReason:        v.YankReason,
			SHA256:            v.DigestSHA256,
			PackageType:       v.PackageType,
			CreatedAt:         v.CreatedAt.Format(time.RFC3339),
			CompatibleWith:    compatibleWithPlatforms(v.AgentCompatibility),
		}
		out = append(out, rv)
	}
	return out
}

// ChangePassword handles authenticated password change requests.
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}
	if len(req.NewPassword) < 8 {
		WriteError(w, http.StatusBadRequest, "WEAK_PASSWORD", "New password must be at least 8 characters")
		return
	}
	if err := h.auth.GetUserRepo().ChangePassword(r.Context(), user.ID, req.CurrentPassword, req.NewPassword); err != nil {
		WriteError(w, http.StatusBadRequest, "CHANGE_PASSWORD_FAILED", err.Error())
		return
	}
	h.logAudit(r, user.Username, "password_change", user.Username)
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// AdminListUsers returns all users (admin only)
func (h *Handler) AdminListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.auth.GetUserRepo().ListUsers(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"users": users})
}

// AdminCreateUser creates a new user (admin only)
func (h *Handler) AdminCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username      string `json:"username"`
		Email         string `json:"email"`
		Password      string `json:"password"`
		Role          string `json:"role"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
		return
	}
	if req.Role == "" {
		req.Role = "user"
	}
	user, err := h.auth.GetUserRepo().CreateUserWithOptions(r.Context(), auth.UserCreateOptions{
		Username:      req.Username,
		Email:         req.Email,
		Password:      req.Password,
		Role:          req.Role,
		EmailVerified: req.EmailVerified || req.Role == "admin",
	})
	if err != nil {
		WriteError(w, http.StatusBadRequest, "CREATE_FAILED", err.Error())
		return
	}
	h.logAudit(r, auth.ActorFromContext(r.Context()), "user_create", req.Username)
	resp := map[string]interface{}{"user": user}
	if h.config.Auth.RequireEmailVerification && user.Email != "" && !user.EmailVerified {
		token, err := h.auth.GetUserRepo().CreateEmailVerificationToken(r.Context(), user.ID, 24*time.Hour)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "VERIFICATION_TOKEN_FAILED", err.Error())
			return
		}
		resp["verification_expires_at"] = token.ExpiresAt
		if h.email.Enabled() {
			if err := h.email.SendVerification(user.Email, user.Username, token.Token); err != nil {
				WriteError(w, http.StatusInternalServerError, "VERIFICATION_EMAIL_FAILED", err.Error())
				return
			}
			resp["verification_email_sent"] = true
		} else {
			resp["verification_token"] = token.Token
			resp["verification_url"] = h.email.VerificationURL(token.Token)
		}
	}
	WriteJSON(w, http.StatusCreated, resp)
}

// AdminSendVerification creates and sends a fresh verification email for a user.
func (h *Handler) AdminSendVerification(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	user, err := h.auth.GetUserRepo().GetUser(r.Context(), username)
	if err != nil {
		WriteError(w, http.StatusNotFound, "USER_NOT_FOUND", err.Error())
		return
	}
	if user.Email == "" {
		WriteError(w, http.StatusBadRequest, "EMAIL_MISSING", "User has no email address")
		return
	}
	if user.EmailVerified {
		WriteJSON(w, http.StatusOK, map[string]interface{}{"status": "already_verified", "user": user.Username})
		return
	}
	token, err := h.auth.GetUserRepo().CreateEmailVerificationToken(r.Context(), user.ID, 24*time.Hour)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "VERIFICATION_TOKEN_FAILED", err.Error())
		return
	}
	resp := map[string]interface{}{
		"user":                    user.Username,
		"verification_expires_at": token.ExpiresAt,
	}
	if h.email.Enabled() {
		if err := h.email.SendVerification(user.Email, user.Username, token.Token); err != nil {
			WriteError(w, http.StatusInternalServerError, "VERIFICATION_EMAIL_FAILED", err.Error())
			return
		}
		resp["verification_email_sent"] = true
	} else {
		resp["verification_token"] = token.Token
		resp["verification_url"] = h.email.VerificationURL(token.Token)
	}
	h.logAudit(r, auth.ActorFromContext(r.Context()), "email_verification_send", username)
	WriteJSON(w, http.StatusOK, resp)
}

// AdminUpdateUserRole updates a user's role (admin only)
func (h *Handler) AdminUpdateUserRole(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	var req struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
		return
	}
	if err := h.auth.GetUserRepo().UpdateUserRole(r.Context(), username, req.Role); err != nil {
		WriteError(w, http.StatusBadRequest, "UPDATE_FAILED", err.Error())
		return
	}
	h.logAudit(r, auth.ActorFromContext(r.Context()), "user_role_update", username)
	WriteJSON(w, http.StatusOK, map[string]string{"username": username, "role": req.Role})
}

// AdminDeleteUser deletes a user (admin only)
func (h *Handler) AdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	actor := auth.ActorFromContext(r.Context())
	if actor == username {
		WriteError(w, http.StatusBadRequest, "SELF_DELETE", "cannot delete your own account")
		return
	}
	if err := h.auth.GetUserRepo().DeleteUser(r.Context(), username); err != nil {
		WriteError(w, http.StatusBadRequest, "DELETE_FAILED", err.Error())
		return
	}
	h.logAudit(r, actor, "user_delete", username)
	w.WriteHeader(http.StatusNoContent)
}
