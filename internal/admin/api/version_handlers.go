package api

import (
	"log/slog"
	"net/http"
	"strconv"
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

	result, err := h.lifecycle.Rollback(r.Context(), pluginID, versionID)
	if err != nil {
		slog.Error("admin: failed to rollback plugin", "id", pluginID, "version_id", versionID, "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":     result.Status,
		"version":    result.Version,
		"version_id": versionID,
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
