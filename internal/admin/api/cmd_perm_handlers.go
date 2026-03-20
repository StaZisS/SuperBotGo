package api

import (
	"encoding/json"
	"net/http"
)

type CommandPermHandler struct {
	store CommandPermStore
}

func NewCommandPermHandler(store CommandPermStore) *CommandPermHandler {
	return &CommandPermHandler{store: store}
}

func (h *CommandPermHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/plugins/{id}/commands/settings", h.handleListSettings)
	mux.HandleFunc("PUT /api/admin/plugins/{id}/commands/{cmd}/enabled", h.handleSetEnabled)
	mux.HandleFunc("PUT /api/admin/plugins/{id}/commands/{cmd}/policy", h.handleSetPolicy)
}

func (h *CommandPermHandler) handleListSettings(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusOK, []CommandSetting{})
		return
	}
	pluginID := r.PathValue("id")
	settings, err := h.store.ListCommandSettings(r.Context(), pluginID)
	if err != nil {
		writeJSON(w, http.StatusOK, []CommandSetting{})
		return
	}
	if settings == nil {
		settings = []CommandSetting{}
	}
	writeJSON(w, http.StatusOK, settings)
}

func (h *CommandPermHandler) handleSetEnabled(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "requires PostgreSQL")
		return
	}
	pluginID := r.PathValue("id")
	cmd := r.PathValue("cmd")

	var body struct {
		Enabled bool `json:"enabled"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := h.store.SetCommandEnabled(r.Context(), pluginID, cmd, body.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update command setting")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (h *CommandPermHandler) handleSetPolicy(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "requires PostgreSQL")
		return
	}
	pluginID := r.PathValue("id")
	cmd := r.PathValue("cmd")

	var body struct {
		Expression string `json:"expression"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := h.store.SetPolicyExpression(r.Context(), pluginID, cmd, body.Expression); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save policy expression")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}
