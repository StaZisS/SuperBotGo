package api

import (
	"net/http"
	"time"

	"SuperBotGo/internal/wasm/registry"
)

func (h *AdminHandler) handleRegistryList(w http.ResponseWriter, r *http.Request) {
	reg := h.loader.Registry()
	if reg == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	type registryEntryResponse struct {
		ID           string                `json:"id"`
		Name         string                `json:"name"`
		Dependencies []registry.Dependency `json:"dependencies,omitempty"`
		Signature    string                `json:"signature,omitempty"`
		Versions     int                   `json:"version_count"`
		Latest       string                `json:"latest_version,omitempty"`
	}

	entries := reg.ListAll()
	result := make([]registryEntryResponse, 0, len(entries))
	for _, e := range entries {
		resp := registryEntryResponse{
			ID:           e.ID,
			Name:         e.Name,
			Dependencies: e.Dependencies,
			Signature:    e.Signature,
			Versions:     len(e.Versions),
		}
		if len(e.Versions) > 0 {
			resp.Latest = e.Versions[0].Version
		}
		result = append(result, resp)
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *AdminHandler) handleRegistryVersions(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")

	reg := h.loader.Registry()
	if reg == nil {
		writeError(w, http.StatusNotFound, "registry not configured")
		return
	}

	versions, err := reg.ListVersions(pluginID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	type versionResponse struct {
		Version       string    `json:"version"`
		WasmHash      string    `json:"wasm_hash"`
		UploadedAt    time.Time `json:"uploaded_at"`
		Changelog     string    `json:"changelog,omitempty"`
		MinSDKVersion int       `json:"min_sdk_version,omitempty"`
	}

	result := make([]versionResponse, 0, len(versions))
	for _, v := range versions {
		result = append(result, versionResponse{
			Version:       v.Version,
			WasmHash:      v.WasmHash,
			UploadedAt:    v.UploadedAt,
			Changelog:     v.Changelog,
			MinSDKVersion: v.MinSDKVersion,
		})
	}

	writeJSON(w, http.StatusOK, result)
}
