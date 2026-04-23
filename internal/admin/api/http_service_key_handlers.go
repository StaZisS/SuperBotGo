package api

import (
	"net/http"
	"strconv"
	"time"
)

type ServiceKeyHandler struct {
	store ServiceKeyStore
}

func NewServiceKeyHandler(store ServiceKeyStore) *ServiceKeyHandler {
	return &ServiceKeyHandler{store: store}
}

func (h *ServiceKeyHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/http/service-keys", h.handleList)
	mux.HandleFunc("POST /api/admin/http/service-keys", h.handleCreate)
	mux.HandleFunc("DELETE /api/admin/http/service-keys/{id}", h.handleDelete)
}

func (h *ServiceKeyHandler) handleList(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusOK, []ServiceKeyRecord{})
		return
	}
	keys, err := h.store.ListServiceKeys(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list service keys")
		return
	}
	writeJSON(w, http.StatusOK, keys)
}

func (h *ServiceKeyHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "requires PostgreSQL")
		return
	}

	var body struct {
		Name      string            `json:"name"`
		Scopes    []ServiceKeyScope `json:"scopes"`
		ExpiresAt string            `json:"expires_at"`
	}
	if !decodeJSONBody(w, r, &body) {
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(body.Scopes) == 0 {
		writeError(w, http.StatusBadRequest, "at least one scope is required")
		return
	}

	var expiresAt *time.Time
	if body.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, body.ExpiresAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "expires_at must be RFC3339")
			return
		}
		expiresAt = &t
	}

	rec, err := h.store.CreateServiceKey(r.Context(), body.Name, body.Scopes, expiresAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create service key")
		return
	}
	writeJSON(w, http.StatusCreated, rec)
}

func (h *ServiceKeyHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "requires PostgreSQL")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid service key ID")
		return
	}

	if err := h.store.DeleteServiceKey(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
