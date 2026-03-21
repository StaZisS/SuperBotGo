package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/pubsub"
	"SuperBotGo/internal/state"
	"SuperBotGo/internal/wasm/adapter"
	"SuperBotGo/internal/wasm/hostapi"
	wasmrt "SuperBotGo/internal/wasm/runtime"
)

const maxUploadSize = 50 << 20

const maxRequestBodySize = 1 << 20

type uploadResponse struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Commands     []wasmrt.CommandDef    `json:"commands"`
	Permissions  []wasmrt.PermissionDef `json:"permissions"`
	ConfigSchema json.RawMessage        `json:"config_schema"`
	WasmKey      string                 `json:"wasm_key"`
	WasmHash     string                 `json:"wasm_hash"`
}

type installResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Status  string `json:"status"`
}

type cmdInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type StateManagerRegistrar interface {
	RegisterCommand(def *state.CommandDefinition)
	UnregisterCommand(name string)
}

type AdminHandler struct {
	store    PluginStore
	blobs    BlobStore
	loader   *adapter.Loader
	manager  *plugin.Manager
	rt       *wasmrt.Runtime
	hostAPI  *hostapi.HostAPI
	stateMgr StateManagerRegistrar
	cmdStore CommandPermStore
	versions VersionStore
	apiKey   string
	bus      *pubsub.Bus
}

func NewAdminHandler(
	store PluginStore,
	blobs BlobStore,
	loader *adapter.Loader,
	manager *plugin.Manager,
	rt *wasmrt.Runtime,
	hostAPI *hostapi.HostAPI,
	stateMgr StateManagerRegistrar,
	cmdStore CommandPermStore,
	versions VersionStore,
	apiKey string,
	bus *pubsub.Bus,
) *AdminHandler {
	if apiKey == "" {
		slog.Warn("admin: API key is not set — admin endpoints are unprotected!")
	}
	return &AdminHandler{
		store:    store,
		blobs:    blobs,
		loader:   loader,
		manager:  manager,
		rt:       rt,
		hostAPI:  hostAPI,
		stateMgr: stateMgr,
		cmdStore: cmdStore,
		versions: versions,
		apiKey:   apiKey,
		bus:      bus,
	}
}

func (h *AdminHandler) publish(ctx context.Context, eventType, pluginID string) {
	if h.bus == nil {
		return
	}
	if err := h.bus.Publish(ctx, pubsub.AdminEvent{
		Type:     eventType,
		PluginID: pluginID,
	}); err != nil {
		slog.Error("admin: failed to publish event", "type", eventType, "plugin", pluginID, "error", err)
	}
}

func (h *AdminHandler) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.apiKey == "" {
			next(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(auth, prefix) {
			writeError(w, http.StatusUnauthorized, "missing or invalid Authorization header")
			return
		}
		token := auth[len(prefix):]
		if subtle.ConstantTimeCompare([]byte(token), []byte(h.apiKey)) != 1 {
			writeError(w, http.StatusForbidden, "invalid API key")
			return
		}
		next(w, r)
	}
}

func (h *AdminHandler) registerPluginCommands(p plugin.Plugin) {
	if h.stateMgr == nil {
		return
	}
	for _, def := range p.Commands() {
		h.stateMgr.RegisterCommand(def)
	}
}

func (h *AdminHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/admin/plugins/upload", h.handleUpload)
	mux.HandleFunc("POST /api/admin/plugins/{id}/install", h.handleInstall)
	mux.HandleFunc("PUT /api/admin/plugins/{id}/config", h.handleUpdateConfig)
	mux.HandleFunc("POST /api/admin/plugins/{id}/update", h.handleUpdate)
	mux.HandleFunc("POST /api/admin/plugins/{id}/disable", h.handleDisable)
	mux.HandleFunc("POST /api/admin/plugins/{id}/enable", h.handleEnable)
	mux.HandleFunc("DELETE /api/admin/plugins/{id}", h.handleDelete)
	mux.HandleFunc("GET /api/admin/plugins/{id}", h.handleGetPlugin)
	mux.HandleFunc("GET /api/admin/plugins", h.handleListPlugins)

	// Version management
	mux.HandleFunc("GET /api/admin/plugins/{id}/versions", h.handleListVersions)
	mux.HandleFunc("POST /api/admin/plugins/{id}/versions/{versionId}/rollback", h.handleRollback)
	mux.HandleFunc("DELETE /api/admin/plugins/{id}/versions/{versionId}", h.handleDeleteVersion)
}

func validateBlobKey(key string) bool {
	if key == "" {
		return false
	}
	if strings.Contains(key, "..") ||
		strings.Contains(key, "/") ||
		strings.Contains(key, "\\") ||
		strings.ContainsAny(key, "\x00") {
		return false
	}
	return true
}

func (h *AdminHandler) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or invalid multipart form")
		return
	}

	file, header, err := r.FormFile("wasm")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing 'wasm' file in form")
		return
	}
	defer file.Close()

	if !strings.HasSuffix(header.Filename, ".wasm") {
		writeError(w, http.StatusBadRequest, "file must have .wasm extension")
		return
	}

	wasmBytes, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read uploaded file")
		return
	}

	compiled, err := h.rt.CompileModule(r.Context(), wasmBytes)
	if err != nil {
		slog.Error("admin: invalid wasm module", "error", err)
		writeError(w, http.StatusBadRequest, "invalid wasm module")
		return
	}

	const probeID = "_upload_probe"
	h.hostAPI.ForPlugin(probeID, nil)
	compiled.ID = probeID

	meta, err := compiled.CallMeta(r.Context())
	h.hostAPI.RevokePermissions(probeID)
	_ = compiled.Close(r.Context())
	if err != nil {
		slog.Error("admin: failed to read plugin metadata", "error", err)
		writeError(w, http.StatusBadRequest, "failed to read plugin metadata")
		return
	}

	wasmKey := fmt.Sprintf("%s_%s.wasm", meta.ID, meta.Version)
	if err := h.blobs.Put(r.Context(), wasmKey, bytes.NewReader(wasmBytes), int64(len(wasmBytes))); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save wasm file")
		return
	}

	hash := sha256.Sum256(wasmBytes)

	writeJSON(w, http.StatusOK, uploadResponse{
		ID:           meta.ID,
		Name:         meta.Name,
		Version:      meta.Version,
		Commands:     meta.Commands,
		Permissions:  meta.Permissions,
		ConfigSchema: meta.ConfigSchema,
		WasmKey:      wasmKey,
		WasmHash:     hex.EncodeToString(hash[:]),
	})
}

func (h *AdminHandler) handleInstall(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")

	var body struct {
		Config      json.RawMessage `json:"config"`
		Permissions []string        `json:"permissions"`
		WasmKey     string          `json:"wasm_key"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
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

	wp, err := h.loader.LoadPluginFromBytes(r.Context(), wasmBytes, body.Config, body.Permissions)
	if err != nil {
		slog.Error("admin: failed to load plugin", "id", pluginID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load plugin")
		return
	}

	if wp.ID() != pluginID {
		_ = h.loader.UnloadPlugin(r.Context(), wp.ID())
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
		Permissions: body.Permissions,
		Enabled:     true,
		WasmHash:    hex.EncodeToString(hash[:]),
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := h.store.SavePlugin(r.Context(), record); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save plugin record")
		return
	}

	if h.versions != nil {
		if _, err := h.versions.SaveVersion(r.Context(), VersionRecord{
			PluginID:    wp.ID(),
			Version:     wp.Version(),
			WasmKey:     body.WasmKey,
			WasmHash:    record.WasmHash,
			ConfigJSON:  body.Config,
			Permissions: body.Permissions,
			Changelog:   "initial install",
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

func (h *AdminHandler) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")

	var body struct {
		Config json.RawMessage `json:"config"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	record, err := h.store.GetPlugin(r.Context(), pluginID)
	if err != nil {
		writeError(w, http.StatusNotFound, "plugin not found")
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

func (h *AdminHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")

	record, err := h.store.GetPlugin(r.Context(), pluginID)
	if err != nil {
		writeError(w, http.StatusNotFound, "plugin not found")
		return
	}

	var oldCommands map[string]struct{}
	if oldPlugin, ok := h.manager.Get(pluginID); ok {
		oldCommands = make(map[string]struct{}, len(oldPlugin.Commands()))
		for _, def := range oldPlugin.Commands() {
			oldCommands[def.Name] = struct{}{}
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or invalid form")
		return
	}

	file, header, err := r.FormFile("wasm")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing 'wasm' file")
		return
	}
	defer file.Close()

	if !strings.HasSuffix(header.Filename, ".wasm") {
		writeError(w, http.StatusBadRequest, "file must have .wasm extension")
		return
	}

	wasmBytes, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read file")
		return
	}

	newKey := fmt.Sprintf("%s_update_%d.wasm", pluginID, time.Now().Unix())
	if err := h.blobs.Put(r.Context(), newKey, bytes.NewReader(wasmBytes), int64(len(wasmBytes))); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save wasm file")
		return
	}

	if err := h.loader.ReloadPluginFromBytes(r.Context(), pluginID, wasmBytes, record.ConfigJSON); err != nil {
		_ = h.blobs.Delete(r.Context(), newKey)
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
		h.syncCommandsOnUpdate(r.Context(), pluginID, oldCommands, wp)
		h.syncPermissionsOnUpdate(pluginID, record, wp)

		if h.versions != nil {
			if _, err := h.versions.SaveVersion(r.Context(), VersionRecord{
				PluginID:    pluginID,
				Version:     wp.Version(),
				WasmKey:     newKey,
				WasmHash:    record.WasmHash,
				ConfigJSON:  record.ConfigJSON,
				Permissions: record.Permissions,
			}); err != nil {
				slog.Error("admin: failed to save version record on update", "plugin", pluginID, "error", err)
			}
		}
	}

	h.publish(r.Context(), pubsub.EventPluginUpdated, pluginID)

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *AdminHandler) syncCommandsOnUpdate(ctx context.Context, pluginID string, oldCommands map[string]struct{}, newPlugin plugin.Plugin) {
	newCommands := make(map[string]struct{}, len(newPlugin.Commands()))
	for _, def := range newPlugin.Commands() {
		newCommands[def.Name] = struct{}{}
	}

	h.registerPluginCommands(newPlugin)

	var removed []string
	for name := range oldCommands {
		if _, ok := newCommands[name]; !ok {
			removed = append(removed, name)
		}
	}

	var added []string
	for name := range newCommands {
		if _, ok := oldCommands[name]; !ok {
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
			slog.Error("admin: failed to delete orphaned command settings",
				"plugin", pluginID, "commands", removed, "error", err)
		} else {
			slog.Info("admin: cleaned up command settings for removed commands",
				"plugin", pluginID, "removed", removed)
		}
	}

	if len(added) > 0 {
		slog.Info("admin: new commands detected (no access settings configured yet)",
			"plugin", pluginID, "added", added)
	}
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

	wp, err := h.loader.LoadPluginFromBytes(r.Context(), wasmBytes, record.ConfigJSON, record.Permissions)
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

func (h *AdminHandler) handleListPlugins(w http.ResponseWriter, r *http.Request) {
	allPlugins := h.manager.All()
	records, _ := h.store.ListPlugins(r.Context())

	type pluginInfo struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Version  string `json:"version"`
		Type     string `json:"type"`
		Status   string `json:"status"`
		Commands int    `json:"commands"`
	}

	result := make([]pluginInfo, 0, len(allPlugins)+len(records))

	for id, p := range allPlugins {
		pType := "go"
		if _, ok := p.(*adapter.WasmPlugin); ok {
			pType = "wasm"
		}
		result = append(result, pluginInfo{
			ID:       id,
			Name:     p.Name(),
			Version:  p.Version(),
			Type:     pType,
			Status:   "active",
			Commands: len(p.Commands()),
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
		resp["permissions"] = record.Permissions
		resp["wasm_hash"] = record.WasmHash
		resp["installed_at"] = record.InstalledAt
		resp["updated_at"] = record.UpdatedAt
		if !record.Enabled {
			resp["status"] = "disabled"
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *AdminHandler) syncPermissionsOnUpdate(pluginID string, record PluginRecord, wp *adapter.WasmPlugin) {
	meta := wp.Meta()

	currentPerms := make(map[string]bool, len(record.Permissions))
	for _, p := range record.Permissions {
		currentPerms[p] = true
	}

	changed := false
	for _, decl := range meta.Permissions {
		if decl.Required && !currentPerms[decl.Key] {
			currentPerms[decl.Key] = true
			changed = true
			slog.Info("admin: auto-granted new required permission on update",
				"plugin", pluginID, "permission", decl.Key)
		}
	}

	if changed {
		newPerms := make([]string, 0, len(currentPerms))
		for p := range currentPerms {
			newPerms = append(newPerms, p)
		}
		record.Permissions = newPerms
		record.UpdatedAt = time.Now()
		ctx := context.Background()
		if err := h.store.SavePlugin(ctx, record); err != nil {
			slog.Error("admin: failed to save updated permissions", "plugin", pluginID, "error", err)
			return
		}
		h.hostAPI.GrantPermissions(pluginID, newPerms)
		h.loader.UpdatePermissions(pluginID, newPerms)
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("admin: failed to encode JSON response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	slog.Warn("admin: API error", "status", status, "message", message)
	writeJSON(w, status, map[string]string{"error": message})
}
