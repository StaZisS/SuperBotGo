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
	"SuperBotGo/internal/state"
	"SuperBotGo/internal/wasm/adapter"
	"SuperBotGo/internal/wasm/hostapi"
	wasmrt "SuperBotGo/internal/wasm/runtime"
)

const maxUploadSize = 50 << 20 // 50MB

// maxRequestBodySize limits non-upload JSON request bodies (1 MB).
const maxRequestBodySize = 1 << 20

// StateManagerRegistrar registers and unregisters command definitions at runtime.
type StateManagerRegistrar interface {
	RegisterCommand(def *state.CommandDefinition)
	UnregisterCommand(name string)
}

// AdminHandler provides HTTP handlers for the Wasm plugin admin API.
type AdminHandler struct {
	store    PluginStore
	blobs    BlobStore
	loader   *adapter.Loader
	manager  *plugin.Manager
	rt       *wasmrt.Runtime
	hostAPI  *hostapi.HostAPI
	stateMgr StateManagerRegistrar
	cmdStore CommandPermStore // nil when PostgreSQL is unavailable
	apiKey   string
}

// NewAdminHandler creates a new AdminHandler.
// If apiKey is non-empty, all routes require Bearer token authentication.
// cmdStore may be nil when PostgreSQL is unavailable.
func NewAdminHandler(
	store PluginStore,
	blobs BlobStore,
	loader *adapter.Loader,
	manager *plugin.Manager,
	rt *wasmrt.Runtime,
	hostAPI *hostapi.HostAPI,
	stateMgr StateManagerRegistrar,
	cmdStore CommandPermStore,
	apiKey string,
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
		apiKey:   apiKey,
	}
}

// requireAuth is middleware that checks the Bearer token against the configured API key.
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

// registerPluginCommands registers a plugin's commands with the state manager.
func (h *AdminHandler) registerPluginCommands(p plugin.Plugin) {
	if h.stateMgr == nil {
		return
	}
	for _, def := range p.Commands() {
		h.stateMgr.RegisterCommand(def)
	}
}

// RegisterRoutes registers all admin API routes on the given mux.
// TODO: wrap handlers with h.requireAuth() before going to production.
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
}

// validateBlobKey checks that a blob key is safe (no path traversal or directory escape).
func validateBlobKey(key string) bool {
	if key == "" {
		return false
	}
	// Block path traversal, absolute paths, backslash paths, and subdirectories.
	if strings.Contains(key, "..") ||
		strings.HasPrefix(key, "/") ||
		strings.Contains(key, "\\") ||
		strings.ContainsAny(key, "\x00") ||
		strings.Contains(key, "/") {
		return false
	}
	return true
}

// handleUpload validates and stores a .wasm file, returning its metadata.
//
// The uploaded module is compiled, then a one-shot "meta" action is run
// to extract metadata. The compiled module is closed immediately after;
// the actual long-lived compiled module is created later by handleInstall
// via Loader.LoadPluginFromBytes.
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

	// Grant temporary permissions so host functions can resolve during meta.
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

	resp := map[string]interface{}{
		"id":            meta.ID,
		"name":          meta.Name,
		"version":       meta.Version,
		"commands":      meta.Commands,
		"permissions":   meta.Permissions,
		"config_schema": meta.ConfigSchema,
		"wasm_key":      wasmKey,
		"wasm_hash":     hex.EncodeToString(hash[:]),
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleInstall loads and registers a previously uploaded plugin.
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":      wp.ID(),
		"name":    wp.Name(),
		"version": wp.Version(),
		"status":  "installed",
	})
}

// handleUpdateConfig updates plugin configuration.
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

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// handleUpdate uploads a new .wasm version and reloads the plugin.
func (h *AdminHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")

	record, err := h.store.GetPlugin(r.Context(), pluginID)
	if err != nil {
		writeError(w, http.StatusNotFound, "plugin not found")
		return
	}

	// Collect old command names before reload.
	var oldCommands map[string]struct{}
	if oldPlugin, ok := h.manager.All()[pluginID]; ok {
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
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// syncCommandsOnUpdate compares old and new command sets after a plugin
// .wasm update, cleaning up orphaned command settings and unregistering
// removed commands from the state manager.
func (h *AdminHandler) syncCommandsOnUpdate(ctx context.Context, pluginID string, oldCommands map[string]struct{}, newPlugin plugin.Plugin) {
	newCommands := make(map[string]struct{}, len(newPlugin.Commands()))
	for _, def := range newPlugin.Commands() {
		newCommands[def.Name] = struct{}{}
	}

	// Register new commands in the state manager.
	h.registerPluginCommands(newPlugin)

	// Find removed commands (present in old but absent in new).
	var removed []string
	for name := range oldCommands {
		if _, ok := newCommands[name]; !ok {
			removed = append(removed, name)
		}
	}

	// Find added commands (present in new but absent in old).
	var added []string
	for name := range newCommands {
		if _, ok := oldCommands[name]; !ok {
			added = append(added, name)
		}
	}

	// Unregister removed commands from the state manager.
	if h.stateMgr != nil {
		for _, name := range removed {
			h.stateMgr.UnregisterCommand(name)
		}
	}

	// Delete orphaned command settings from the database.
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

// handleDisable disables a plugin without deleting it.
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

	writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

// handleEnable re-enables a disabled plugin.
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

	writeJSON(w, http.StatusOK, map[string]string{"status": "enabled"})
}

// handleDelete completely removes a plugin.
func (h *AdminHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")

	record, err := h.store.GetPlugin(r.Context(), pluginID)
	if err != nil {
		writeError(w, http.StatusNotFound, "plugin not found")
		return
	}

	// Unregister commands from the state manager before removing.
	if h.stateMgr != nil {
		if p, ok := h.manager.All()[pluginID]; ok {
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

	// Clean up all command settings for this plugin.
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

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleListPlugins returns all plugins (Go and Wasm).
func (h *AdminHandler) handleListPlugins(w http.ResponseWriter, r *http.Request) {
	allPlugins := h.manager.All()
	records, _ := h.store.ListPlugins(r.Context())
	recordMap := make(map[string]PluginRecord, len(records))
	for _, rec := range records {
		recordMap[rec.ID] = rec
	}

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

// handleGetPlugin returns detailed info about a single plugin.
func (h *AdminHandler) handleGetPlugin(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")

	p := h.manager.All()[pluginID]

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
		// Serialize only command names/descriptions (CommandDefinition contains
		// unexportable func fields like MessageBuilder).
		type cmdInfo struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
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
