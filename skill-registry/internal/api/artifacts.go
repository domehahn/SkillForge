package api

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/skillforge/skill-registry/internal/auth"
)

func (h *Handler) ListArtifacts(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	artifacts, total, err := h.registry.ListArtifacts(r.Context(), r.URL.Query().Get("kind"), r.URL.Query().Get("q"), r.URL.Query().Get("namespace"), limit, offset)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "LIST_FAILED", err.Error())
		return
	}
	visible := artifacts[:0]
	for _, artifact := range artifacts {
		if artifact.Visibility == "public" || h.hasNamespaceRole(r, artifact.Namespace, "reader") {
			visible = append(visible, artifact)
		}
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"artifacts": visible, "total": total, "limit": limit, "offset": offset})
}

func (h *Handler) GetArtifact(w http.ResponseWriter, r *http.Request) {
	artifact, versions, tags, err := h.registry.GetArtifact(r.Context(), chi.URLParam(r, "kind"), chi.URLParam(r, "namespace"), chi.URLParam(r, "name"))
	if err != nil || artifact == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Artifact not found")
		return
	}
	if artifact.Visibility != "public" && !h.requireNamespaceRole(w, r, artifact.Namespace, "reader") {
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"artifact": artifact, "versions": versions, "dist_tags": tags})
}

func (h *Handler) GetArtifactVersion(w http.ResponseWriter, r *http.Request) {
	if !h.requireArtifactRead(w, r) {
		return
	}
	version, err := h.registry.ResolveArtifactVersion(r.Context(), chi.URLParam(r, "kind"), chi.URLParam(r, "namespace"), chi.URLParam(r, "name"), chi.URLParam(r, "version"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, version)
}

func (h *Handler) PublishArtifact(w http.ResponseWriter, r *http.Request) {
	if !h.requireNamespaceRole(w, r, chi.URLParam(r, "namespace"), "maintainer") {
		return
	}
	packageType := packageTypeFromContentType(r.Header.Get("Content-Type"))
	if packageType == "" {
		WriteError(w, http.StatusBadRequest, "INVALID_CONTENT_TYPE", "Content-Type must be application/gzip or application/zip")
		return
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, int64(h.config.Storage.MaxPackageSizeMB)*1024*1024+1))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "READ_ERROR", err.Error())
		return
	}
	result, err := h.registry.PublishArtifact(
		r.Context(), chi.URLParam(r, "kind"), chi.URLParam(r, "namespace"), chi.URLParam(r, "name"),
		chi.URLParam(r, "version"), data, packageType, auth.ActorFromContext(r.Context()),
	)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "already exists") {
			status = http.StatusConflict
		}
		WriteError(w, status, "PUBLISH_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusCreated, result)
}

func (h *Handler) DownloadArtifact(w http.ResponseWriter, r *http.Request) {
	if !h.requireArtifactRead(w, r) {
		return
	}
	data, version, err := h.registry.DownloadArtifact(r.Context(), chi.URLParam(r, "kind"), chi.URLParam(r, "namespace"), chi.URLParam(r, "name"), chi.URLParam(r, "version"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	contentType, extension := "application/gzip", "tgz"
	if version.PackageType == "zip" {
		contentType, extension = "application/zip", "zip"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Artifact-Digest-SHA256", version.DigestSHA256)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s.%s"`, version.Name, version.Version, extension))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) ArtifactGraph(w http.ResponseWriter, r *http.Request) {
	if !h.requireArtifactRead(w, r) {
		return
	}
	graph, err := h.registry.ArtifactGraph(r.Context(), chi.URLParam(r, "kind"), chi.URLParam(r, "namespace"), chi.URLParam(r, "name"), chi.URLParam(r, "version"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "GRAPH_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, graph)
}

func (h *Handler) ArtifactLockfile(w http.ResponseWriter, r *http.Request) {
	if !h.requireArtifactRead(w, r) {
		return
	}
	version, err := h.registry.ResolveArtifactVersion(r.Context(), chi.URLParam(r, "kind"), chi.URLParam(r, "namespace"), chi.URLParam(r, "name"), chi.URLParam(r, "version"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, version.Lockfile)
}

func (h *Handler) PromoteArtifact(w http.ResponseWriter, r *http.Request) {
	if !h.requireNamespaceRole(w, r, chi.URLParam(r, "namespace"), "maintainer") {
		return
	}
	var req struct {
		Version string `json:"version"`
		Channel string `json:"channel"`
	}
	if err := ParseJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.registry.PromoteArtifact(r.Context(), chi.URLParam(r, "kind"), chi.URLParam(r, "namespace"), chi.URLParam(r, "name"), req.Version, req.Channel, auth.ActorFromContext(r.Context())); err != nil {
		WriteError(w, http.StatusBadRequest, "PROMOTION_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"version": req.Version, "channel": req.Channel})
}

func (h *Handler) ListPromotions(w http.ResponseWriter, r *http.Request) {
	promotions, err := h.registry.ListPromotions(r.Context(), chi.URLParam(r, "kind"), chi.URLParam(r, "namespace"), chi.URLParam(r, "name"))
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"promotions": promotions})
}

func (h *Handler) AttestArtifact(w http.ResponseWriter, r *http.Request) {
	if !h.requireNamespaceRole(w, r, chi.URLParam(r, "namespace"), "maintainer") {
		return
	}
	var req struct {
		SignatureStatus string `json:"signature_status"`
		ScanStatus      string `json:"scan_status"`
	}
	if err := ParseJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.registry.SetArtifactAttestation(r.Context(), chi.URLParam(r, "kind"), chi.URLParam(r, "namespace"), chi.URLParam(r, "name"), chi.URLParam(r, "version"), req.SignatureStatus, req.ScanStatus); err != nil {
		WriteError(w, http.StatusBadRequest, "ATTESTATION_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *Handler) CreateArtifactAttestation(w http.ResponseWriter, r *http.Request) {
	if !h.requireNamespaceRole(w, r, chi.URLParam(r, "namespace"), "maintainer") {
		return
	}
	var req struct {
		Type      string                 `json:"type"`
		Digest    string                 `json:"digest"`
		Predicate map[string]interface{} `json:"predicate"`
	}
	if err := ParseJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	attestation, err := h.registry.CreateArtifactAttestation(r.Context(), chi.URLParam(r, "kind"), chi.URLParam(r, "namespace"), chi.URLParam(r, "name"), chi.URLParam(r, "version"), req.Type, req.Digest, auth.ActorFromContext(r.Context()), req.Predicate)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "ATTESTATION_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusCreated, attestation)
}

func (h *Handler) ListArtifactAttestations(w http.ResponseWriter, r *http.Request) {
	if !h.requireArtifactRead(w, r) {
		return
	}
	attestations, err := h.registry.ListArtifactAttestations(r.Context(), chi.URLParam(r, "kind"), chi.URLParam(r, "namespace"), chi.URLParam(r, "name"), chi.URLParam(r, "version"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "LIST_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"attestations": attestations})
}

func (h *Handler) ListNamespaceMembers(w http.ResponseWriter, r *http.Request) {
	members, err := h.registry.ListNamespaceMembers(r.Context(), chi.URLParam(r, "namespace"))
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"members": members})
}

func (h *Handler) UpsertNamespaceMember(w http.ResponseWriter, r *http.Request) {
	if !h.requireNamespaceRole(w, r, chi.URLParam(r, "namespace"), "owner") {
		return
	}
	var req struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	if err := ParseJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	member, err := h.registry.UpsertNamespaceMember(r.Context(), chi.URLParam(r, "namespace"), req.Username, req.Role)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "MEMBER_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, member)
}

func (h *Handler) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	hooks, err := h.registry.ListWebhooks(r.Context(), chi.URLParam(r, "namespace"))
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"webhooks": hooks})
}

func (h *Handler) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	if !h.requireNamespaceRole(w, r, chi.URLParam(r, "namespace"), "owner") {
		return
	}
	var req struct {
		URL    string   `json:"url"`
		Events []string `json:"events"`
	}
	if err := ParseJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	hook, err := h.registry.CreateWebhook(r.Context(), chi.URLParam(r, "namespace"), req.URL, req.Events)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "WEBHOOK_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusCreated, hook)
}

func (h *Handler) requireNamespaceRole(w http.ResponseWriter, r *http.Request, namespace, role string) bool {
	user := auth.UserFromContext(r.Context())
	if user != nil && user.Role == "admin" {
		return true
	}
	allowed, err := h.registry.AuthorizeNamespace(r.Context(), namespace, auth.ActorFromContext(r.Context()), role)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "ACL_FAILED", "Failed to evaluate namespace permissions")
		return false
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient namespace role")
		return false
	}
	return true
}

func (h *Handler) hasNamespaceRole(r *http.Request, namespace, role string) bool {
	user := auth.UserFromContext(r.Context())
	if user != nil && user.Role == "admin" {
		return true
	}
	allowed, err := h.registry.AuthorizeNamespace(r.Context(), namespace, auth.ActorFromContext(r.Context()), role)
	return err == nil && allowed
}

func (h *Handler) requireArtifactRead(w http.ResponseWriter, r *http.Request) bool {
	artifact, _, _, err := h.registry.GetArtifact(r.Context(), chi.URLParam(r, "kind"), chi.URLParam(r, "namespace"), chi.URLParam(r, "name"))
	if err != nil || artifact == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Artifact not found")
		return false
	}
	if artifact.Visibility == "public" {
		return true
	}
	return h.requireNamespaceRole(w, r, artifact.Namespace, "reader")
}

func packageTypeFromContentType(contentType string) string {
	switch contentType {
	case "application/gzip", "application/x-gzip":
		return "tgz"
	case "application/zip":
		return "zip"
	default:
		return ""
	}
}
