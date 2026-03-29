package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/pubsub"
	"SuperBotGo/internal/wasm/adapter"
)

func (h *AdminHandler) handleUpload(w http.ResponseWriter, r *http.Request) {
	wasmBytes, ok := readWasmFromForm(w, r)
	if !ok {
		return
	}

	compiled, err := h.rt.CompileModule(r.Context(), wasmBytes)
	if err != nil {
		slog.Error("admin: invalid wasm module", "error", err)
		writeError(w, http.StatusBadRequest, "invalid wasm module")
		return
	}

	const probeID = "_upload_probe"
	h.hostAPI.GrantPermissions(probeID, nil)
	compiled.ID = probeID

	meta, err := compiled.CallMeta(r.Context())
	h.hostAPI.RevokePermissions(probeID)
	if closeErr := compiled.Close(r.Context()); closeErr != nil {
		slog.Warn("admin: failed to close compiled module", "error", closeErr)
	}
	if err != nil {
		slog.Error("admin: failed to read plugin metadata", "error", err)
		writeError(w, http.StatusBadRequest, "failed to read plugin metadata")
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

	hash := sha256.Sum256(wasmBytes)

	writeJSON(w, http.StatusOK, uploadResponse{
		ID:              meta.ID,
		Name:            meta.Name,
		Version:         meta.Version,
		Triggers:        meta.Triggers,
		Requirements:    meta.Requirements,
		ConfigSchema:    meta.ConfigSchema,
		WasmKey:         wasmKey,
		WasmHash:        hex.EncodeToString(hash[:]),
		ExistingVersion: existingVersion,
	})
}

func (h *AdminHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")

	record, err := h.store.GetPlugin(r.Context(), pluginID)
	if err != nil {
		writeError(w, http.StatusNotFound, "plugin not found")
		return
	}

	oldTriggers := collectConfigurableTriggers(h.manager, pluginID)

	wasmBytes, ok := readWasmFromForm(w, r)
	if !ok {
		return
	}

	newKey := fmt.Sprintf("%s_update_%d.wasm", pluginID, time.Now().Unix())
	if err := h.blobs.Put(r.Context(), newKey, bytes.NewReader(wasmBytes), int64(len(wasmBytes))); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save wasm file")
		return
	}

	if err := h.loader.ReloadPluginFromBytes(r.Context(), pluginID, wasmBytes, record.ConfigJSON); err != nil {
		if delErr := h.blobs.Delete(r.Context(), newKey); delErr != nil {
			slog.Warn("admin: failed to clean up wasm blob after reload failure", "key", newKey, "error", delErr)
		}
		slog.Error("admin: failed to reload plugin", "id", pluginID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to reload plugin")
		return
	}

	hash := sha256.Sum256(wasmBytes)
	record.WasmKey = newKey
	record.WasmHash = hex.EncodeToString(hash[:])
	record.UpdatedAt = time.Now()
	if err := h.store.SavePlugin(r.Context(), record); err != nil {
		slog.Error("admin: failed to update plugin record after reload", "error", err)
	}

	h.manager.Remove(pluginID)
	if wp, ok := h.loader.GetPlugin(pluginID); ok {
		h.manager.Register(wp)
		h.syncTriggersOnUpdate(r.Context(), pluginID, oldTriggers, wp)

		if h.versions != nil {
			if _, err := h.versions.SaveVersion(r.Context(), VersionRecord{
				PluginID:   pluginID,
				Version:    wp.Version(),
				WasmKey:    newKey,
				WasmHash:   record.WasmHash,
				ConfigJSON: record.ConfigJSON,
			}); err != nil {
				slog.Error("admin: failed to save version record on update", "plugin", pluginID, "error", err)
			}
		}
	}

	h.publish(r.Context(), pubsub.EventPluginUpdated, pluginID)

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// collectConfigurableTriggers returns names of all non-cron triggers for a plugin.
func collectConfigurableTriggers(mgr *plugin.Manager, pluginID string) map[string]struct{} {
	p, ok := mgr.Get(pluginID)
	if !ok {
		return nil
	}
	if wp, ok := p.(*adapter.WasmPlugin); ok {
		triggers := make(map[string]struct{})
		for _, t := range wp.Meta().Triggers {
			if t.Type != "cron" {
				triggers[t.Name] = struct{}{}
			}
		}
		return triggers
	}
	triggers := make(map[string]struct{}, len(p.Commands()))
	for _, def := range p.Commands() {
		triggers[def.Name] = struct{}{}
	}
	return triggers
}

func (h *AdminHandler) syncTriggersOnUpdate(ctx context.Context, pluginID string, oldTriggers map[string]struct{}, newPlugin plugin.Plugin) {
	newTriggers := collectConfigurableTriggers(h.manager, pluginID)

	h.registerPluginCommands(newPlugin)

	var removed []string
	for name := range oldTriggers {
		if _, ok := newTriggers[name]; !ok {
			removed = append(removed, name)
		}
	}

	var added []string
	for name := range newTriggers {
		if _, ok := oldTriggers[name]; !ok {
			added = append(added, name)
		}
	}

	if h.stateMgr != nil {
		for _, name := range removed {
			h.stateMgr.UnregisterCommand(name)
		}
	}

	if h.cmdStore != nil && len(removed) > 0 {
		if err := h.cmdStore.DeleteCommandSettings(ctx, pluginID, removed); err != nil {
			slog.Error("admin: failed to delete orphaned trigger settings",
				"plugin", pluginID, "triggers", removed, "error", err)
		} else {
			slog.Info("admin: cleaned up settings for removed triggers",
				"plugin", pluginID, "removed", removed)
		}
	}

	if len(added) > 0 {
		slog.Info("admin: new triggers detected (no access settings configured yet)",
			"plugin", pluginID, "added", added)
	}
}
