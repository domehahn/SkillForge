package api

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/skillforge/skill-registry/internal/auth"
	"github.com/skillforge/skill-registry/internal/config"
	"github.com/skillforge/skill-registry/internal/registry"
)

// Handler handles HTTP requests
type Handler struct {
	registry *registry.Registry
	auth     *auth.Authenticator
	logger   *slog.Logger
	config   *config.Config
}

// NewHandler creates a new API handler
func NewHandler(reg *registry.Registry, authenticator *auth.Authenticator, logger *slog.Logger, cfg *config.Config) *Handler {
	return &Handler{
		registry: reg,
		auth:     authenticator,
		logger:   logger,
		config:   cfg,
	}
}

// RegisterRoutes registers all API routes
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Health endpoints
	r.Get("/healthz", h.HealthCheck)
	r.Get("/readyz", h.ReadyCheck)

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/metadata", h.GetMetadata)

		// Auth endpoints (no authentication required)
		r.Post("/auth/login", h.Login)

		// Public read endpoints (no auth required)
		r.Group(func(r chi.Router) {
			r.Use(h.auth.Middleware())
			r.Get("/skills", h.ListSkills)
			r.Get("/skills/{namespace}/{name}", h.GetSkill)
			r.Get("/skills/{namespace}/{name}/versions/{version}", h.GetVersion)
			r.Get("/skills/{namespace}/{name}/versions/{version}/download", h.Download)
		})

		// Token management endpoints (require auth)
		r.Group(func(r chi.Router) {
			r.Use(h.auth.Middleware("write"))
			r.Post("/tokens", h.CreateToken)
			r.Get("/tokens", h.ListTokens)
			r.Delete("/tokens/{id}", h.RevokeToken)
		})

		// Write endpoints (require auth with write scope)
		r.Group(func(r chi.Router) {
			r.Use(h.auth.Middleware("write"))
			r.Put("/skills/{namespace}/{name}/versions/{version}", h.Publish)
			r.Post("/validate", h.Validate)
		})

		// Delete endpoints (require auth with delete scope)
		r.Group(func(r chi.Router) {
			r.Use(h.auth.Middleware("delete"))
			r.Delete("/skills/{namespace}/{name}/versions/{version}", h.DeleteVersion)
		})
	})

	// Serve static files from web UI (if exists)
	h.ServeStaticFiles(r)
}

// HealthCheck handles health check requests
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ReadyCheck handles readiness check requests
func (h *Handler) ReadyCheck(w http.ResponseWriter, r *http.Request) {
	// Could check database connectivity here
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// GetMetadata returns registry metadata
func (h *Handler) GetMetadata(w http.ResponseWriter, r *http.Request) {
	authMode := "disabled"
	if h.config.Auth.Enabled {
		authMode = "token"
	}

	metadata := RegistryMetadata{
		Name:                 "Skill Registry",
		Version:              "1.0.0",
		SupportedPackageTypes: h.config.Storage.AllowedPackageTypes,
		AuthMode:             authMode,
		Capabilities:         []string{"publish", "list", "download", "validate", "delete"},
	}

	WriteJSON(w, http.StatusOK, metadata)
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

	// Read package data
	data, err := io.ReadAll(io.LimitReader(r.Body, int64(h.config.Storage.MaxPackageSizeMB)*1024*1024+1))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "READ_ERROR", "Failed to read request body")
		return
	}

	// Publish
	actor := auth.ActorFromContext(ctx)
	skillVersion, err := h.registry.Publish(ctx, namespace, name, version, data, packageType, registry.PublishOptions{
		Force:     force,
		CreatedBy: actor,
		Source:    "local",
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

	WriteJSON(w, http.StatusCreated, skillVersion)
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

	WriteJSON(w, http.StatusOK, skills)
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

	response := map[string]interface{}{
		"skill":    skill,
		"versions": versions,
	}

	WriteJSON(w, http.StatusOK, response)
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
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("X-Skill-Digest-SHA256", skillVersion.DigestSHA256)
	filename := fmt.Sprintf("%s-%s-%s.tgz", namespace, name, version)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))

	w.WriteHeader(http.StatusOK)
	w.Write(data)
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

	// Determine scopes based on role
	scopes := []string{"read", "write"}
	if user.Role == "admin" {
		scopes = append(scopes, "delete")
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
		"token":      token.Token,
		"user":       user.Username,
		"role":       user.Role,
		"scopes":     scopes,
		"expires_at": token.ExpiresAt,
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

	// Validate scopes
	allowedScopes := []string{"read", "write"}
	if user.Role == "admin" {
		allowedScopes = append(allowedScopes, "delete")
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

	WriteJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
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
