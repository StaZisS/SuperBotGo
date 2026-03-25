package api

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"SuperBotGo/internal/pubsub"
)

func (h *AdminHandler) handleListVersions(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")

	if h.versions == nil {
		writeJSON(w, http.StatusOK, []VersionRecord{})
		return
	}

	if _, err := h.store.GetPlugin(r.Context(), pluginID); err != nil {
		writeError(w, http.StatusNotFound, "plugin not found")
		return
	}

	versions, err := h.versions.ListVersions(r.Context(), pluginID)
	if err != nil {
		slog.Error("admin: failed to list versions", "plugin", pluginID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list versions")
		return
	}

	if versions == nil {
		versions = []VersionRecord{}
	}

	writeJSON(w, http.StatusOK, versions)
}

func (h *AdminHandler) handleRollback(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")
	versionIDStr := r.PathValue("versionId")

	versionID, err := strconv.ParseInt(versionIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid version ID")
		return
	}

	if h.versions == nil {
		writeError(w, http.StatusNotImplemented, "version store not configured")
		return
	}

	record, err := h.store.GetPlugin(r.Context(), pluginID)
	if err != nil {
		writeError(w, http.StatusNotFound, "plugin not found")
		return
	}

	ver, err := h.versions.GetVersion(r.Context(), versionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "version not found")
		return
	}

	if ver.PluginID != pluginID {
		writeError(w, http.StatusBadRequest, "version does not belong to this plugin")
		return
	}

	rc, err := h.blobs.Get(r.Context(), ver.WasmKey)
	if err != nil {
		slog.Error("admin: rollback blob not found", "key", ver.WasmKey, "error", err)
		writeError(w, http.StatusInternalServerError, "wasm binary for this version is no longer available")
		return
	}
	wasmBytes, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read wasm blob")
		return
	}

	hash := sha256.Sum256(wasmBytes)
	actualHash := hex.EncodeToString(hash[:])
	if actualHash != ver.WasmHash {
		slog.Warn("admin: rollback hash mismatch", "expected", ver.WasmHash, "actual", actualHash)
		writeError(w, http.StatusInternalServerError, "wasm binary integrity check failed")
		return
	}

	var oldCommands map[string]struct{}
	if oldPlugin, ok := h.manager.Get(pluginID); ok {
		oldCommands = make(map[string]struct{}, len(oldPlugin.Commands()))
		for _, def := range oldPlugin.Commands() {
			oldCommands[def.Name] = struct{}{}
		}
	}

	if err := h.loader.ReloadPluginFromBytes(r.Context(), pluginID, wasmBytes, ver.ConfigJSON); err != nil {
		slog.Error("admin: failed to reload plugin on rollback", "id", pluginID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load plugin from version")
		return
	}

	record.WasmKey = ver.WasmKey
	record.WasmHash = ver.WasmHash
	record.ConfigJSON = ver.ConfigJSON
	record.Enabled = true
	record.UpdatedAt = time.Now()
	if err := h.store.SavePlugin(r.Context(), record); err != nil {
		slog.Error("admin: failed to save plugin record after rollback", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to save plugin record")
		return
	}

	h.manager.Remove(pluginID)
	if wp, ok := h.loader.GetPlugin(pluginID); ok {
		h.manager.Register(wp)
		h.syncCommandsOnUpdate(r.Context(), pluginID, oldCommands, wp)

		// Permissions are auto-derived from requirements during reload.
	}

	h.publish(r.Context(), pubsub.EventPluginUpdated, pluginID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "rolled_back",
		"version":    ver.Version,
		"version_id": ver.ID,
	})
}

func (h *AdminHandler) handleDeleteVersion(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")
	versionIDStr := r.PathValue("versionId")

	versionID, err := strconv.ParseInt(versionIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid version ID")
		return
	}

	if h.versions == nil {
		writeError(w, http.StatusNotImplemented, "version store not configured")
		return
	}

	record, err := h.store.GetPlugin(r.Context(), pluginID)
	if err != nil {
		writeError(w, http.StatusNotFound, "plugin not found")
		return
	}

	ver, err := h.versions.GetVersion(r.Context(), versionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "version not found")
		return
	}

	if ver.PluginID != pluginID {
		writeError(w, http.StatusBadRequest, "version does not belong to this plugin")
		return
	}

	if ver.WasmKey == record.WasmKey {
		writeError(w, http.StatusConflict, "cannot delete the currently active version")
		return
	}

	blobInUse := false
	if allVersions, listErr := h.versions.ListVersions(r.Context(), pluginID); listErr == nil {
		for _, v := range allVersions {
			if v.ID != versionID && v.WasmKey == ver.WasmKey {
				blobInUse = true
				break
			}
		}
	}
	if !blobInUse {
		if err := h.blobs.Delete(r.Context(), ver.WasmKey); err != nil {
			slog.Warn("admin: failed to delete version blob", "key", ver.WasmKey, "error", err)
		}
	}

	if err := h.versions.DeleteVersion(r.Context(), versionID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete version")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
