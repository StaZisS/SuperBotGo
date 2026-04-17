package api

import (
	"encoding/json"
	"net/http"
)

func (h *AdminHandler) handleInstall(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")

	var body struct {
		Config  json.RawMessage `json:"config"`
		WasmKey string          `json:"wasm_key"`
	}
	if !decodeJSONBody(w, r, &body) {
		return
	}
	if body.WasmKey == "" {
		writeError(w, http.StatusBadRequest, "wasm_key is required")
		return
	}
	if !validateBlobKey(body.WasmKey) {
		writeError(w, http.StatusBadRequest, "invalid wasm_key")
		return
	}

	result, err := h.lifecycle.Install(r.Context(), pluginID, body.WasmKey, body.Config)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, installResponse{
		ID:      result.PluginID,
		Name:    result.Name,
		Version: result.Version,
		Status:  result.Status,
	})
}

func (h *AdminHandler) handleEnable(w http.ResponseWriter, r *http.Request) {
	result, err := h.lifecycle.Enable(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "plugin not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": result.Status})
}

func (h *AdminHandler) handleDisable(w http.ResponseWriter, r *http.Request) {
	result, err := h.lifecycle.Disable(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "plugin not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": result.Status})
}

func (h *AdminHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	result, err := h.lifecycle.Delete(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "plugin not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": result.Status})
}
