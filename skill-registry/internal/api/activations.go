package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/skillforge/skill-registry/internal/auth"
)

func (h *Handler) ListActivations(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		WriteError(w, http.StatusBadRequest, "NAMESPACE_REQUIRED", "namespace query parameter is required")
		return
	}
	if !h.requireNamespaceRole(w, r, namespace, "reader") {
		return
	}
	items, err := h.registry.ListActivations(r.Context(), namespace)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"activations": items})
}

func (h *Handler) GetActivation(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	if !h.requireNamespaceRole(w, r, namespace, "reader") {
		return
	}
	a, events, err := h.registry.GetActivation(r.Context(), namespace, chi.URLParam(r, "kind"), chi.URLParam(r, "name"))
	if err != nil || a == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "activation not found")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"activation": a, "events": events})
}

func (h *Handler) ChangeActivation(w http.ResponseWriter, r *http.Request) {
	if !h.config.Activation.Enabled {
		WriteError(w, http.StatusServiceUnavailable, "ACTIVATION_DISABLED", "artifact activation is disabled")
		return
	}
	namespace := chi.URLParam(r, "namespace")
	if !h.requireNamespaceRole(w, r, namespace, "maintainer") {
		return
	}
	var req struct {
		Action  string `json:"action"`
		Version string `json:"version"`
		Reason  string `json:"reason"`
	}
	if err := ParseJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	a, err := h.registry.ChangeActivation(r.Context(), namespace, chi.URLParam(r, "kind"), chi.URLParam(r, "name"), req.Version, req.Action, req.Reason, auth.ActorFromContext(r.Context()), h.config.Activation.RequireVerifiedSignatures)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "ACTIVATION_REJECTED", err.Error())
		return
	}
	h.logAudit(r, auth.ActorFromContext(r.Context()), "activation."+req.Action, a.Kind+"/"+namespace+"/"+a.Name+"@"+a.Version)
	WriteJSON(w, http.StatusOK, a)
}

func (h *Handler) ReportActivation(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	if !h.requireNamespaceRole(w, r, namespace, "maintainer") {
		return
	}
	var req struct {
		ObservedState string `json:"observed_state"`
		LastError     string `json:"last_error"`
		Generation    int64  `json:"generation"`
	}
	if err := ParseJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	a, err := h.registry.ReportActivation(r.Context(), namespace, chi.URLParam(r, "kind"), chi.URLParam(r, "name"), req.ObservedState, req.LastError, auth.ActorFromContext(r.Context()), req.Generation)
	if err != nil {
		WriteError(w, http.StatusConflict, "REPORT_REJECTED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, a)
}
