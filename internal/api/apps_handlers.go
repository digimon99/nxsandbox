package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/digimon99/nxsandbox/internal/apps"
	"github.com/digimon99/nxsandbox/internal/auth"
	"github.com/go-chi/chi/v5"
)

type AppsHandlers struct {
	store *apps.Store
}

func NewAppsHandlers(store *apps.Store) *AppsHandlers {
	return &AppsHandlers{store: store}
}

type createAppRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (h *AppsHandlers) ListApps(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r)
	items, err := h.store.ListAppsByUser(r.Context(), claims.Subject)
	if err != nil {
		Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list apps")
		return
	}
	Success(w, items)
}

func (h *AppsHandlers) CreateApp(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r)
	var req createAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Slug = strings.TrimSpace(strings.ToLower(req.Slug))
	if req.Name == "" || req.Slug == "" {
		Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "name and slug are required")
		return
	}
	app, err := h.store.CreateApp(r.Context(), claims.Subject, req.Name, req.Slug)
	if err != nil {
		Error(w, http.StatusBadRequest, "CREATE_APP_FAILED", err.Error())
		return
	}
	JSON(w, http.StatusCreated, Response{Success: true, Data: app})
}

func (h *AppsHandlers) ListDeployments(w http.ResponseWriter, r *http.Request) {
	appID := strings.TrimSpace(chi.URLParam(r, "id"))
	if appID == "" {
		Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "app id is required")
		return
	}
	items, err := h.store.ListDeploymentsByApp(r.Context(), appID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list deployments")
		return
	}
	Success(w, items)
}
