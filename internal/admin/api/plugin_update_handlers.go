package api

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"

	wasmrt "SuperBotGo/internal/wasm/runtime"
)

func (h *AdminHandler) handleUpload(w http.ResponseWriter, r *http.Request) {
	wasmBytes, ok := readWasmFromForm(w, r)
	if !ok {
		return
	}

	meta, err := h.probeUploadedPlugin(r.Context(), wasmBytes)
	if err != nil {
		slog.Error("admin: invalid wasm module", "error", err)
		writeError(w, http.StatusBadRequest, "invalid wasm module")
		return
	}

	var existingVersion string
	if wp, ok := h.loader.GetPlugin(meta.ID); ok {
		existingVersion = wp.Version()
	} else if h.versions != nil {
		if vv, err := h.versions.ListVersions(r.Context(), meta.ID); err == nil && len(vv) > 0 {
			existingVersion = vv[0].Version
		}
	}

	wasmKey := fmt.Sprintf("%s_%s.wasm", meta.ID, meta.Version)
	if err := h.blobs.Put(r.Context(), wasmKey, bytes.NewReader(wasmBytes), int64(len(wasmBytes))); err != nil {
		slog.Error("admin: failed to save wasm file", "key", wasmKey, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to save wasm file")
		return
	}

	writeJSON(w, http.StatusOK, uploadResponse{
		ID:              meta.ID,
		Name:            meta.Name,
		Version:         meta.Version,
		RPCMethods:      meta.RPCMethods,
		Triggers:        meta.Triggers,
		Requirements:    meta.Requirements,
		ConfigSchema:    meta.ConfigSchema,
		WasmKey:         wasmKey,
		WasmHash:        hashWASM(wasmBytes),
		ExistingVersion: existingVersion,
	})
}

func (h *AdminHandler) handleUpdatePreview(w http.ResponseWriter, r *http.Request) {
	wasmBytes, ok := readWasmFromForm(w, r)
	if !ok {
		return
	}

	preview, err := h.buildUpdatePreview(r.Context(), r.PathValue("id"), wasmBytes)
	if err != nil {
		slog.Error("admin: failed to build plugin update preview", "id", r.PathValue("id"), "error", err)
		if _, storeErr := h.store.GetPlugin(r.Context(), r.PathValue("id")); storeErr != nil {
			writeError(w, http.StatusNotFound, "plugin not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, preview)
}

func (h *AdminHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	wasmBytes, ok := readWasmFromForm(w, r)
	if !ok {
		return
	}
	result, err := h.lifecycle.Update(r.Context(), r.PathValue("id"), wasmBytes)
	if err != nil {
		slog.Error("admin: failed to update plugin", "id", r.PathValue("id"), "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": result.Status})
}

func (h *AdminHandler) probeUploadedPlugin(ctx context.Context, wasmBytes []byte) (wasmrt.PluginMeta, error) {
	if h.loader == nil {
		return wasmrt.PluginMeta{}, fmt.Errorf("wasm loader is not configured")
	}
	meta, err := h.loader.ProbeMetadataFromBytes(ctx, wasmBytes)
	if err != nil {
		return wasmrt.PluginMeta{}, fmt.Errorf("probe uploaded plugin metadata: %w", err)
	}
	return meta, nil
}
