package api

import (
	"encoding/json"
	"net/http"
	"time"

	"SuperBotGo/internal/pubsub"
	"SuperBotGo/internal/wasm/adapter"
)

func (h *AdminHandler) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")

	var body struct {
		Config json.RawMessage `json:"config"`
	}
	if !decodeJSONBody(w, r, &body) {
		return
	}

	record, err := h.store.GetPlugin(r.Context(), pluginID)
	if err != nil {
		writeError(w, http.StatusNotFound, "plugin not found")
		return
	}

	if err := h.loader.ValidateConfig(pluginID, body.Config); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	record.ConfigJSON = body.Config
	record.UpdatedAt = time.Now()
	if err := h.store.SavePlugin(r.Context(), record); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update plugin record")
		return
	}

	if wp, ok := h.loader.GetPlugin(pluginID); ok {
		wp.SetConfig(body.Config)
	}

	h.publish(r.Context(), pubsub.EventConfigChanged, pluginID)

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *AdminHandler) handleValidateConfig(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")

	var body struct {
		Config json.RawMessage `json:"config"`
	}
	if !decodeJSONBody(w, r, &body) {
		return
	}

	if err := h.loader.ValidateConfig(pluginID, body.Config); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"valid": "true"})
}

func (h *AdminHandler) handleListPlugins(w http.ResponseWriter, r *http.Request) {
	allPlugins := h.manager.All()
	records, _ := h.store.ListPlugins(r.Context())

	type pluginInfo struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Version  string `json:"version"`
		Type     string `json:"type"`
		Status   string `json:"status"`
		Triggers int    `json:"triggers"`
	}

	result := make([]pluginInfo, 0, len(allPlugins)+len(records))

	for id, p := range allPlugins {
		pType := "go"
		triggerCount := len(p.Commands())
		if wp, ok := p.(*adapter.WasmPlugin); ok {
			pType = "wasm"
			triggerCount = len(wp.Meta().Triggers)
		}
		result = append(result, pluginInfo{
			ID:       id,
			Name:     p.Name(),
			Version:  p.Version(),
			Type:     pType,
			Status:   "active",
			Triggers: triggerCount,
		})
	}

	for _, rec := range records {
		if _, active := allPlugins[rec.ID]; !active {
			result = append(result, pluginInfo{
				ID:     rec.ID,
				Type:   "wasm",
				Status: "disabled",
			})
		}
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *AdminHandler) handleGetPlugin(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")

	p, _ := h.manager.Get(pluginID)

	record, storeErr := h.store.GetPlugin(r.Context(), pluginID)

	if p == nil && storeErr != nil {
		writeError(w, http.StatusNotFound, "plugin not found")
		return
	}

	resp := map[string]interface{}{"id": pluginID}

	if p != nil {
		pType := "go"
		if wp, ok := p.(*adapter.WasmPlugin); ok {
			pType = "wasm"
			resp["meta"] = wp.Meta()
		}
		resp["name"] = p.Name()
		resp["version"] = p.Version()
		resp["type"] = pType
		resp["status"] = "active"
		cmds := make([]cmdInfo, 0, len(p.Commands()))
		for _, def := range p.Commands() {
			cmds = append(cmds, cmdInfo{Name: def.Name, Description: def.Description})
		}
		resp["commands"] = cmds
	}

	if storeErr == nil {
		resp["config"] = record.ConfigJSON
		resp["wasm_hash"] = record.WasmHash
		resp["installed_at"] = record.InstalledAt
		resp["updated_at"] = record.UpdatedAt
		if !record.Enabled {
			resp["status"] = "disabled"
		}
	}

	writeJSON(w, http.StatusOK, resp)
}
