package api

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/skillforge/skill-registry/internal/auth"
	"github.com/skillforge/skill-registry/internal/metadata"
)

func (h *Handler) ListArtifacts(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	sort := r.URL.Query().Get("sort")
	verified := r.URL.Query().Get("verified") == "true"
	artifacts, total, err := h.registry.ListArtifacts(r.Context(), r.URL.Query().Get("kind"), r.URL.Query().Get("q"), r.URL.Query().Get("namespace"), sort, limit, offset, verified)
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

func (h *Handler) GetNamespace(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	artifacts, total, err := h.registry.ListArtifacts(r.Context(), "", "", namespace, "updated", 100, 0, false)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}
	artifactCount, totalPulls, err := h.registry.NamespaceStats(r.Context(), namespace)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "STATS_FAILED", err.Error())
		return
	}
	bio := h.registry.GetNamespaceBio(r.Context(), namespace)
	verified := h.registry.GetNamespaceVerified(r.Context(), namespace)
	displayName := h.registry.GetDisplayName(r.Context(), namespace)
	pinned := h.registry.GetPinnedArtifacts(r.Context(), namespace)
	members, _ := h.registry.ListNamespaceMembers(r.Context(), namespace)
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"namespace":      namespace,
		"artifact_count": artifactCount,
		"total_pulls":    totalPulls,
		"artifacts":      artifacts,
		"total":          total,
		"bio":            bio,
		"verified":       verified,
		"display_name":   displayName,
		"pinned":         pinned,
		"member_count":   len(members),
	})
}

func (h *Handler) GetFacets(w http.ResponseWriter, r *http.Request) {
	facets, err := h.registry.KindFacets(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "FACETS_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"facets": facets})
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	snap, err := h.registry.Metrics(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "STATS_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, snap)
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

func (h *Handler) RemoveNamespaceMember(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	username := chi.URLParam(r, "username")
	if !h.requireNamespaceRole(w, r, namespace, "owner") {
		return
	}
	if err := h.registry.RemoveNamespaceMember(r.Context(), namespace, username); err != nil {
		WriteError(w, http.StatusInternalServerError, "REMOVE_FAILED", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

func (h *Handler) GetArtifactStats(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days <= 0 || days > 365 {
		days = 30
	}
	trend, err := h.registry.GetArtifactDownloadTrend(r.Context(), kind, namespace, name, days)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "STATS_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"trend": trend, "days": days})
}

func (h *Handler) PatchArtifactVersion(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	version := chi.URLParam(r, "version")
	if !h.requireNamespaceRole(w, r, namespace, "maintainer") {
		return
	}
	var body struct {
		ReleaseNotes *string `json:"release_notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	if body.ReleaseNotes != nil {
		if err := h.registry.SetVersionReleaseNotes(r.Context(), kind, namespace, name, version, *body.ReleaseNotes); err != nil {
			WriteError(w, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
			return
		}
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"version": version, "release_notes": body.ReleaseNotes})
}

func (h *Handler) TestWebhook(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	webhookIDStr := chi.URLParam(r, "id")
	webhookID, _ := strconv.ParseInt(webhookIDStr, 10, 64)
	if !h.requireNamespaceRole(w, r, namespace, "maintainer") {
		return
	}

	hooks, err := h.registry.ListWebhooks(r.Context(), namespace)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}
	var hookURL string
	for _, h := range hooks {
		if h.ID == webhookID {
			hookURL = h.URL
			break
		}
	}
	if hookURL == "" {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Webhook not found")
		return
	}

	start := time.Now()
	payload := `{"event":"test","namespace":"` + namespace + `","timestamp":"` + start.UTC().Format(time.RFC3339) + `"}`
	resp, doErr := (&http.Client{Timeout: 10 * time.Second}).Post(hookURL, "application/json", strings.NewReader(payload))

	delivery := &metadata.WebhookDelivery{
		WebhookID:  webhookID,
		Namespace:  namespace,
		Event:      "test",
		DurationMs: time.Since(start).Milliseconds(),
	}
	if doErr != nil {
		delivery.StatusCode = 0
		delivery.Success = false
	} else {
		resp.Body.Close()
		delivery.StatusCode = resp.StatusCode
		delivery.Success = resp.StatusCode >= 200 && resp.StatusCode < 300
	}
	_ = h.registry.LogWebhookDelivery(r.Context(), delivery)

	if doErr != nil {
		WriteError(w, http.StatusBadGateway, "DELIVERY_FAILED", doErr.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"status_code": delivery.StatusCode, "success": delivery.Success, "duration_ms": delivery.DurationMs})
}

func (h *Handler) ListWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	webhookIDStr := chi.URLParam(r, "id")
	webhookID, _ := strconv.ParseInt(webhookIDStr, 10, 64)
	if !h.requireNamespaceRole(w, r, namespace, "maintainer") {
		return
	}
	deliveries, err := h.registry.ListWebhookDeliveries(r.Context(), webhookID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"deliveries": deliveries})
}

func (h *Handler) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	if !h.requireNamespaceRole(w, r, namespace, "maintainer") {
		return
	}
	webhookID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := h.registry.DeleteWebhook(r.Context(), webhookID); err != nil {
		WriteError(w, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	hooks, err := h.registry.ListWebhooks(r.Context(), chi.URLParam(r, "namespace"))
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}
	if hooks == nil {
		hooks = []metadata.Webhook{}
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

func (h *Handler) PatchNamespace(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	if !h.requireNamespaceRole(w, r, namespace, "maintainer") {
		return
	}
	actor := auth.ActorFromContext(r.Context())
	user := auth.UserFromContext(r.Context())
	var body struct {
		Bio         *string `json:"bio"`
		Verified    *bool   `json:"verified"`
		DisplayName *string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	if body.Bio != nil {
		if err := h.registry.SetNamespaceBio(r.Context(), namespace, *body.Bio); err != nil {
			WriteError(w, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
			return
		}
	}
	if body.DisplayName != nil {
		if err := h.registry.SetDisplayName(r.Context(), namespace, *body.DisplayName); err != nil {
			WriteError(w, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
			return
		}
	}
	if body.Verified != nil {
		if user == nil || user.Role != "admin" {
			WriteError(w, http.StatusForbidden, "FORBIDDEN", "Only admins can set namespace verification")
			return
		}
		if err := h.registry.SetNamespaceVerified(r.Context(), namespace, *body.Verified); err != nil {
			WriteError(w, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
			return
		}
		_ = actor
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"namespace":    namespace,
		"bio":          h.registry.GetNamespaceBio(r.Context(), namespace),
		"verified":     h.registry.GetNamespaceVerified(r.Context(), namespace),
		"display_name": h.registry.GetDisplayName(r.Context(), namespace),
	})
}

func (h *Handler) GetArtifactDependents(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	dependents, err := h.registry.GetArtifactUsedBy(r.Context(), namespace, name)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"dependents": dependents, "count": len(dependents)})
}

func (h *Handler) ListTopPublishers(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 12
	}
	publishers, err := h.registry.ListTopPublishers(r.Context(), limit)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"publishers": publishers})
}

func (h *Handler) GetPinned(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	pinned := h.registry.GetPinnedArtifacts(r.Context(), namespace)
	WriteJSON(w, http.StatusOK, map[string]interface{}{"pinned": pinned})
}

func (h *Handler) SetPinned(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	if !h.requireNamespaceRole(w, r, namespace, "maintainer") {
		return
	}
	var body struct {
		Pinned []string `json:"pinned"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	if len(body.Pinned) > 6 {
		body.Pinned = body.Pinned[:6]
	}
	if err := h.registry.SetPinnedArtifacts(r.Context(), namespace, body.Pinned); err != nil {
		WriteError(w, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"pinned": body.Pinned})
}

func (h *Handler) AdminListNamespaces(w http.ResponseWriter, r *http.Request) {
	namespaces, err := h.registry.ListAdminNamespaces(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"namespaces": namespaces})
}

func (h *Handler) AdminVerifyNamespace(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	var body struct {
		Verified bool `json:"verified"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	if err := h.registry.SetNamespaceVerified(r.Context(), namespace, body.Verified); err != nil {
		WriteError(w, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"namespace": namespace, "verified": body.Verified})
}

// GetMCPManifest returns a Claude Desktop-compatible MCP server configuration.
func (h *Handler) GetMCPManifest(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	ref := r.URL.Query().Get("version")
	if ref == "" {
		ref = "latest"
	}
	version, err := h.registry.ResolveArtifactVersion(r.Context(), "mcp", namespace, name, ref)
	if err != nil || version == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "MCP artifact not found")
		return
	}
	var mcpSpec *metadata.MCPSpec
	if version.Manifest != nil {
		mcpSpec = version.Manifest.Spec.MCP
	}
	// Build Claude Desktop-compatible server config
	serverKey := namespace + "/" + name
	serverConfig := map[string]interface{}{
		"command": []string{"skforge", "run", serverKey},
		"env":     map[string]string{},
	}
	if mcpSpec != nil {
		if len(mcpSpec.Command) > 0 {
			serverConfig["command"] = mcpSpec.Command
			if len(mcpSpec.Args) > 0 {
				serverConfig["args"] = mcpSpec.Args
			}
		}
		if len(mcpSpec.Env) > 0 {
			serverConfig["env"] = mcpSpec.Env
		}
	}
	tools := []interface{}{}
	resources := []interface{}{}
	prompts := []interface{}{}
	if mcpSpec != nil {
		for _, t := range mcpSpec.Tools {
			tools = append(tools, t)
		}
		for _, res := range mcpSpec.Resources {
			resources = append(resources, res)
		}
		for _, p := range mcpSpec.Prompts {
			prompts = append(prompts, p)
		}
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"name":    serverKey,
		"version": version.Version,
		"transport": func() string {
			if mcpSpec != nil {
				return mcpSpec.Transport
			}
			return "stdio"
		}(),
		"server_config": serverConfig,
		"tools":         tools,
		"resources":     resources,
		"prompts":       prompts,
		"claude_desktop": map[string]interface{}{
			"mcpServers": map[string]interface{}{
				name: serverConfig,
			},
		},
	})
}

func (h *Handler) AtomFeed(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	artifacts, _, err := h.registry.ListArtifacts(r.Context(), "", "", namespace, "updated", 20, 0, false)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	host := r.Host
	if host == "" {
		host = "localhost"
	}
	baseURL := scheme + "://" + host

	type atomEntry struct {
		XMLName xml.Name `xml:"entry"`
		Title   string   `xml:"title"`
		Link    struct {
			Href string `xml:"href,attr"`
		} `xml:"link"`
		ID      string `xml:"id"`
		Updated string `xml:"updated"`
		Summary string `xml:"summary"`
		Author  struct {
			Name string `xml:"name"`
		} `xml:"author"`
	}
	type atomFeed struct {
		XMLName xml.Name    `xml:"feed"`
		Xmlns   string      `xml:"xmlns,attr"`
		Title   string      `xml:"title"`
		Link    string      `xml:"link"`
		SelfURL string      `xml:"-"`
		ID      string      `xml:"id"`
		Updated string      `xml:"updated"`
		Entries []atomEntry `xml:"entry"`
	}

	updated := ""
	entries := make([]atomEntry, 0, len(artifacts))
	for _, a := range artifacts {
		u := a.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z")
		if updated == "" {
			updated = u
		}
		url := fmt.Sprintf("%s/artifacts/%s/%s/%s", baseURL, a.Kind, a.Namespace, a.Name)
		desc := a.Description
		if desc == "" {
			desc = fmt.Sprintf("%s/%s %s", a.Namespace, a.Name, a.LatestVersion)
		}
		e := atomEntry{
			Title:   html.EscapeString(fmt.Sprintf("%s/%s %s", a.Namespace, a.Name, a.LatestVersion)),
			ID:      url,
			Updated: u,
			Summary: html.EscapeString(desc),
		}
		e.Link.Href = url
		e.Author.Name = a.Namespace
		entries = append(entries, e)
	}
	if updated == "" {
		updated = "1970-01-01T00:00:00Z"
	}

	w.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+"\n")
	fmt.Fprintf(w, `<feed xmlns="http://www.w3.org/2005/Atom">`+"\n")
	fmt.Fprintf(w, `  <title>%s — SkillForge</title>`+"\n", html.EscapeString(namespace))
	fmt.Fprintf(w, `  <link href="%s/namespace/%s"/>`, baseURL, namespace)
	fmt.Fprintf(w, `  <link rel="self" type="application/atom+xml" href="%s/api/v1/namespaces/%s/feed"/>`+"\n", baseURL, namespace)
	fmt.Fprintf(w, `  <id>%s/namespace/%s</id>`+"\n", baseURL, namespace)
	fmt.Fprintf(w, `  <updated>%s</updated>`+"\n", updated)
	for _, e := range entries {
		enc := xml.NewEncoder(w)
		enc.Indent("  ", "  ")
		_ = enc.Encode(e)
	}
	fmt.Fprintf(w, "</feed>\n")
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}
	var body struct {
		DisplayName *string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	if body.DisplayName != nil {
		if len(*body.DisplayName) > 100 {
			WriteError(w, http.StatusBadRequest, "INVALID", "Display name must be 100 characters or less")
			return
		}
		if err := h.registry.SetDisplayName(r.Context(), user.Username, *body.DisplayName); err != nil {
			WriteError(w, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
			return
		}
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"display_name": h.registry.GetDisplayName(r.Context(), user.Username),
	})
}

func (h *Handler) PatchArtifact(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	if !h.requireNamespaceRole(w, r, namespace, "maintainer") {
		return
	}
	var body struct {
		Description *string  `json:"description"`
		Readme      *string  `json:"readme"`
		Tags        []string `json:"tags"`
		Visibility  *string  `json:"visibility"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	if err := h.registry.UpdateArtifactMeta(r.Context(), kind, namespace, name, body.Description, body.Readme, body.Tags, body.Visibility); err != nil {
		WriteError(w, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}
	artifact, versions, tags, err := h.registry.GetArtifact(r.Context(), kind, namespace, name)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "GET_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"artifact": artifact, "versions": versions, "dist_tags": tags})
}

func (h *Handler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	actor := auth.ActorFromContext(r.Context())
	if actor == "" {
		WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	notifications, err := h.registry.GetNotifications(r.Context(), actor)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "FETCH_FAILED", err.Error())
		return
	}
	if notifications == nil {
		notifications = []metadata.Notification{}
	}
	unread := 0
	for _, n := range notifications {
		if !n.Read {
			unread++
		}
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"notifications": notifications, "unread": unread})
}

func (h *Handler) MarkNotificationsRead(w http.ResponseWriter, r *http.Request) {
	actor := auth.ActorFromContext(r.Context())
	if actor == "" {
		WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	if err := h.registry.MarkNotificationsRead(r.Context(), actor); err != nil {
		WriteError(w, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) DeleteNotification(w http.ResponseWriter, r *http.Request) {
	actor := auth.ActorFromContext(r.Context())
	if actor == "" {
		WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_ID", "invalid notification id")
		return
	}
	if err := h.registry.DeleteNotification(r.Context(), id, actor); err != nil {
		WriteError(w, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Stars ──────────────────────────────────────────────────────────────────

func (h *Handler) StarArtifact(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	actor := auth.ActorFromContext(r.Context())
	if actor == "" {
		WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	if err := h.registry.StarArtifact(r.Context(), kind, namespace, name, actor); err != nil {
		WriteError(w, http.StatusInternalServerError, "STAR_FAILED", err.Error())
		return
	}
	count, starred := h.registry.GetStarInfo(r.Context(), kind, namespace, name, actor)
	WriteJSON(w, http.StatusOK, map[string]interface{}{"stars": count, "starred": starred})
}

func (h *Handler) UnstarArtifact(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	actor := auth.ActorFromContext(r.Context())
	if actor == "" {
		WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	if err := h.registry.UnstarArtifact(r.Context(), kind, namespace, name, actor); err != nil {
		WriteError(w, http.StatusInternalServerError, "UNSTAR_FAILED", err.Error())
		return
	}
	count, starred := h.registry.GetStarInfo(r.Context(), kind, namespace, name, actor)
	WriteJSON(w, http.StatusOK, map[string]interface{}{"stars": count, "starred": starred})
}

func (h *Handler) GetArtifactStarInfo(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	actor := auth.ActorFromContext(r.Context())
	count, starred := h.registry.GetStarInfo(r.Context(), kind, namespace, name, actor)
	WriteJSON(w, http.StatusOK, map[string]interface{}{"stars": count, "starred": starred})
}

// ── Scan results ───────────────────────────────────────────────────────────

func (h *Handler) ListScanResults(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	version := chi.URLParam(r, "version")
	results, err := h.registry.ListScanResults(r.Context(), kind, namespace, name, version)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "FETCH_FAILED", err.Error())
		return
	}
	if results == nil {
		results = []metadata.ScanResult{}
	}
	// Aggregate severity counts
	counts := map[string]int{"critical": 0, "high": 0, "medium": 0, "low": 0, "unknown": 0}
	for _, r := range results {
		sev := r.Severity
		if _, ok := counts[sev]; !ok {
			sev = "unknown"
		}
		counts[sev]++
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"results": results, "summary": counts})
}

// ── Comments ───────────────────────────────────────────────────────────────

func (h *Handler) ListComments(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	comments, err := h.registry.ListComments(r.Context(), kind, namespace, name)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "FETCH_FAILED", err.Error())
		return
	}
	if comments == nil {
		comments = []metadata.ArtifactComment{}
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"comments": comments})
}

func (h *Handler) AddComment(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	actor := auth.ActorFromContext(r.Context())
	if actor == "" {
		WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	var body struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	body.Body = strings.TrimSpace(body.Body)
	if body.Body == "" {
		WriteError(w, http.StatusBadRequest, "EMPTY_COMMENT", "comment body cannot be empty")
		return
	}
	if len(body.Body) > 4000 {
		WriteError(w, http.StatusBadRequest, "TOO_LONG", "comment body too long (max 4000 chars)")
		return
	}
	c := &metadata.ArtifactComment{Kind: kind, Namespace: namespace, Name: name, Username: actor, Body: body.Body}
	if err := h.registry.AddComment(r.Context(), c); err != nil {
		WriteError(w, http.StatusInternalServerError, "ADD_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusCreated, c)
}

func (h *Handler) UpdateComment(w http.ResponseWriter, r *http.Request) {
	actor := auth.ActorFromContext(r.Context())
	if actor == "" {
		WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_ID", "invalid comment id")
		return
	}
	var body struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	body.Body = strings.TrimSpace(body.Body)
	if body.Body == "" {
		WriteError(w, http.StatusBadRequest, "EMPTY_COMMENT", "comment body cannot be empty")
		return
	}
	if err := h.registry.UpdateComment(r.Context(), id, actor, body.Body); err != nil {
		WriteError(w, http.StatusForbidden, "UPDATE_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	actor := auth.ActorFromContext(r.Context())
	if actor == "" {
		WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_ID", "invalid comment id")
		return
	}
	if err := h.registry.DeleteComment(r.Context(), id, actor); err != nil {
		WriteError(w, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Collections ────────────────────────────────────────────────────────────

func (h *Handler) ListCollections(w http.ResponseWriter, r *http.Request) {
	owner := chi.URLParam(r, "owner")
	cols, err := h.registry.ListCollections(r.Context(), owner)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "FETCH_FAILED", err.Error())
		return
	}
	if cols == nil {
		cols = []metadata.Collection{}
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"collections": cols})
}

func (h *Handler) GetCollection(w http.ResponseWriter, r *http.Request) {
	owner := chi.URLParam(r, "owner")
	slug := chi.URLParam(r, "slug")
	col, err := h.registry.GetCollection(r.Context(), owner, slug)
	if err != nil || col == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "collection not found")
		return
	}
	WriteJSON(w, http.StatusOK, col)
}

func (h *Handler) CreateCollection(w http.ResponseWriter, r *http.Request) {
	actor := auth.ActorFromContext(r.Context())
	if actor == "" {
		WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Visibility  string `json:"visibility"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		WriteError(w, http.StatusBadRequest, "INVALID_NAME", "name is required")
		return
	}
	if body.Visibility == "" {
		body.Visibility = "public"
	}
	slug := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(body.Name), " ", "-"))
	col := &metadata.Collection{
		Owner: actor, Slug: slug, Name: body.Name,
		Description: body.Description, Visibility: body.Visibility,
		Artifacts: []metadata.CollectionArtifact{},
	}
	if err := h.registry.CreateCollection(r.Context(), col); err != nil {
		WriteError(w, http.StatusInternalServerError, "CREATE_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusCreated, col)
}

func (h *Handler) UpdateCollection(w http.ResponseWriter, r *http.Request) {
	actor := auth.ActorFromContext(r.Context())
	owner := chi.URLParam(r, "owner")
	slug := chi.URLParam(r, "slug")
	if actor != owner {
		WriteError(w, http.StatusForbidden, "FORBIDDEN", "you don't own this collection")
		return
	}
	col, _ := h.registry.GetCollection(r.Context(), owner, slug)
	if col == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "collection not found")
		return
	}
	var body struct {
		Name        string                        `json:"name"`
		Description string                        `json:"description"`
		Artifacts   []metadata.CollectionArtifact `json:"artifacts"`
		Visibility  string                        `json:"visibility"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	if body.Name == "" {
		body.Name = col.Name
	}
	if body.Visibility == "" {
		body.Visibility = col.Visibility
	}
	if body.Artifacts == nil {
		body.Artifacts = col.Artifacts
	}
	if err := h.registry.UpdateCollection(r.Context(), owner, slug, body.Name, body.Description, body.Artifacts, body.Visibility); err != nil {
		WriteError(w, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}
	col.Name, col.Description, col.Artifacts, col.Visibility = body.Name, body.Description, body.Artifacts, body.Visibility
	WriteJSON(w, http.StatusOK, col)
}

func (h *Handler) DeleteCollection(w http.ResponseWriter, r *http.Request) {
	actor := auth.ActorFromContext(r.Context())
	owner := chi.URLParam(r, "owner")
	slug := chi.URLParam(r, "slug")
	if actor != owner {
		WriteError(w, http.StatusForbidden, "FORBIDDEN", "you don't own this collection")
		return
	}
	if err := h.registry.DeleteCollection(r.Context(), owner, slug); err != nil {
		WriteError(w, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Insights ───────────────────────────────────────────────────────────────

func (h *Handler) GetNamespaceInsights(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	actor := auth.ActorFromContext(r.Context())
	if actor != namespace && !h.hasNamespaceRole(r, namespace, "member") {
		WriteError(w, http.StatusForbidden, "FORBIDDEN", "insights are private to namespace members")
		return
	}
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days <= 0 || days > 90 {
		days = 30
	}
	insights, err := h.registry.GetNamespaceInsights(r.Context(), namespace, days)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "FETCH_FAILED", err.Error())
		return
	}
	if insights == nil {
		insights = []metadata.ArtifactInsight{}
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"insights": insights, "days": days})
}

// ── Transfer ───────────────────────────────────────────────────────────────

func (h *Handler) TransferArtifact(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	if !h.requireNamespaceRole(w, r, namespace, "owner") {
		return
	}
	var body struct {
		ToNamespace string `json:"to_namespace"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	if strings.TrimSpace(body.ToNamespace) == "" {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "to_namespace is required")
		return
	}
	if err := h.registry.TransferArtifact(r.Context(), kind, namespace, name, body.ToNamespace); err != nil {
		WriteError(w, http.StatusInternalServerError, "TRANSFER_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"status": "transferred", "to_namespace": body.ToNamespace})
}

func (h *Handler) GetFollowInfo(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	actor := auth.ActorFromContext(r.Context())
	following := false
	if actor != "" {
		following = h.registry.IsFollowing(r.Context(), actor, namespace)
	}
	count := h.registry.GetFollowerCount(r.Context(), namespace)
	WriteJSON(w, http.StatusOK, map[string]interface{}{"following": following, "followers": count})
}

func (h *Handler) FollowNamespace(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	actor := auth.ActorFromContext(r.Context())
	if actor == "" {
		WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "login required")
		return
	}
	if actor == namespace {
		WriteError(w, http.StatusBadRequest, "SELF_FOLLOW", "cannot follow yourself")
		return
	}
	if err := h.registry.FollowNamespace(r.Context(), actor, namespace); err != nil {
		WriteError(w, http.StatusInternalServerError, "FOLLOW_FAILED", err.Error())
		return
	}
	count := h.registry.GetFollowerCount(r.Context(), namespace)
	WriteJSON(w, http.StatusOK, map[string]interface{}{"following": true, "followers": count})
}

func (h *Handler) UnfollowNamespace(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	actor := auth.ActorFromContext(r.Context())
	if actor == "" {
		WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "login required")
		return
	}
	if err := h.registry.UnfollowNamespace(r.Context(), actor, namespace); err != nil {
		WriteError(w, http.StatusInternalServerError, "UNFOLLOW_FAILED", err.Error())
		return
	}
	count := h.registry.GetFollowerCount(r.Context(), namespace)
	WriteJSON(w, http.StatusOK, map[string]interface{}{"following": false, "followers": count})
}

func (h *Handler) GetFollowing(w http.ResponseWriter, r *http.Request) {
	actor := auth.ActorFromContext(r.Context())
	if actor == "" {
		WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "login required")
		return
	}
	namespaces, err := h.registry.GetFollowing(r.Context(), actor)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "FOLLOWING_FAILED", err.Error())
		return
	}
	if namespaces == nil {
		namespaces = []string{}
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"following": namespaces})
}
