package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"SuperBotGo/internal/pubsub"
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

	rc, err := h.blobs.Get(r.Context(), body.WasmKey)
	if err != nil {
		writeError(w, http.StatusBadRequest, "wasm blob not found")
		return
	}
	wasmBytes, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read wasm blob")
		return
	}

	wp, err := h.loader.LoadPluginFromBytes(r.Context(), wasmBytes, body.Config)
	if err != nil {
		slog.Error("admin: failed to load plugin", "id", pluginID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load plugin")
		return
	}

	if wp.ID() != pluginID {
		if unloadErr := h.loader.UnloadPlugin(r.Context(), wp.ID()); unloadErr != nil {
			slog.Warn("admin: failed to unload mismatched plugin", "id", wp.ID(), "error", unloadErr)
		}
		slog.Warn("admin: plugin ID mismatch", "url_id", pluginID, "wasm_id", wp.ID())
		writeError(w, http.StatusBadRequest, "plugin ID mismatch")
		return
	}

	h.manager.Register(wp)
	h.registerPluginCommands(wp)

	hash := sha256.Sum256(wasmBytes)

	record := PluginRecord{
		ID:          wp.ID(),
		WasmKey:     body.WasmKey,
		ConfigJSON:  body.Config,
		Enabled:     true,
		WasmHash:    hex.EncodeToString(hash[:]),
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := h.store.SavePlugin(r.Context(), record); err != nil {
		slog.Error("admin: failed to save plugin record", "plugin", record.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to save plugin record")
		return
	}

	if h.versions != nil {
		if _, err := h.versions.SaveVersion(r.Context(), VersionRecord{
			PluginID:   wp.ID(),
			Version:    wp.Version(),
			WasmKey:    body.WasmKey,
			WasmHash:   record.WasmHash,
			ConfigJSON: body.Config,
			Changelog:  "initial install",
		}); err != nil {
			slog.Error("admin: failed to save initial version record", "plugin", wp.ID(), "error", err)
		}
	}

	h.publish(r.Context(), pubsub.EventPluginInstalled, wp.ID())

	writeJSON(w, http.StatusOK, installResponse{
		ID:      wp.ID(),
		Name:    wp.Name(),
		Version: wp.Version(),
		Status:  "installed",
	})
}

func (h *AdminHandler) handleEnable(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")

	record, err := h.store.GetPlugin(r.Context(), pluginID)
	if err != nil {
		writeError(w, http.StatusNotFound, "plugin not found")
		return
	}

	if record.Enabled {
		writeJSON(w, http.StatusOK, map[string]string{"status": "already_enabled"})
		return
	}

	rc, err := h.blobs.Get(r.Context(), record.WasmKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read wasm blob")
		return
	}
	wasmBytes, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read wasm blob")
		return
	}

	wp, err := h.loader.LoadPluginFromBytes(r.Context(), wasmBytes, record.ConfigJSON)
	if err != nil {
		slog.Error("admin: failed to load plugin on enable", "id", pluginID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load plugin")
		return
	}

	h.manager.Register(wp)
	h.registerPluginCommands(wp)

	record.Enabled = true
	record.UpdatedAt = time.Now()
	if err := h.store.SavePlugin(r.Context(), record); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update record")
		return
	}

	h.publish(r.Context(), pubsub.EventPluginEnabled, pluginID)

	writeJSON(w, http.StatusOK, map[string]string{"status": "enabled"})
}

func (h *AdminHandler) handleDisable(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")

	record, err := h.store.GetPlugin(r.Context(), pluginID)
	if err != nil {
		writeError(w, http.StatusNotFound, "plugin not found")
		return
	}

	if !record.Enabled {
		writeJSON(w, http.StatusOK, map[string]string{"status": "already_disabled"})
		return
	}

	if err := h.loader.UnloadPlugin(r.Context(), pluginID); err != nil {
		slog.Warn("admin: error unloading plugin", "id", pluginID, "error", err)
	}
	h.manager.Remove(pluginID)

	record.Enabled = false
	record.UpdatedAt = time.Now()
	if err := h.store.SavePlugin(r.Context(), record); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update record")
		return
	}

	h.publish(r.Context(), pubsub.EventPluginDisabled, pluginID)

	writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

func (h *AdminHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")

	record, err := h.store.GetPlugin(r.Context(), pluginID)
	if err != nil {
		writeError(w, http.StatusNotFound, "plugin not found")
		return
	}

	if h.stateMgr != nil {
		if p, ok := h.manager.Get(pluginID); ok {
			for _, def := range p.Commands() {
				h.stateMgr.UnregisterCommand(def.Name)
			}
		}
	}

	if record.Enabled {
		if err := h.loader.UnloadPlugin(r.Context(), pluginID); err != nil {
			slog.Warn("admin: error unloading plugin during delete", "id", pluginID, "error", err)
		}
	}
	h.manager.Remove(pluginID)

	if record.WasmKey != "" {
		if err := h.blobs.Delete(r.Context(), record.WasmKey); err != nil {
			slog.Warn("admin: failed to delete wasm blob", "key", record.WasmKey, "error", err)
		}
	}

	if h.cmdStore != nil {
		if err := h.cmdStore.DeleteAllPluginCommandSettings(r.Context(), pluginID); err != nil {
			slog.Error("admin: failed to delete command settings on plugin delete",
				"plugin", pluginID, "error", err)
		}
	}

	if err := h.store.DeletePlugin(r.Context(), pluginID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete record")
		return
	}

	h.publish(r.Context(), pubsub.EventPluginUninstalled, pluginID)

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
