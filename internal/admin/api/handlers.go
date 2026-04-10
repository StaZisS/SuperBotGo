package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

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
	ID              string                  `json:"id"`
	Name            string                  `json:"name"`
	Version         string                  `json:"version"`
	Triggers        []wasmrt.TriggerDef     `json:"triggers"`
	Requirements    []wasmrt.RequirementDef `json:"requirements"`
	ConfigSchema    json.RawMessage         `json:"config_schema"`
	WasmKey         string                  `json:"wasm_key"`
	WasmHash        string                  `json:"wasm_hash"`
	ExistingVersion string                  `json:"existing_version,omitempty"`
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
	RegisterCommand(pluginID string, def *state.CommandDefinition)
	UnregisterCommand(pluginID, name string)
	UnregisterAllCommands(pluginID string)
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
	bus *pubsub.Bus,
) *AdminHandler {
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

func (h *AdminHandler) registerPluginCommands(p plugin.Plugin) {
	if h.stateMgr == nil {
		return
	}
	for _, def := range p.Commands() {
		h.stateMgr.RegisterCommand(p.ID(), def)
	}
}

// unregisterPluginCommands drops every state-machine command registered for
// pluginID. If the plugin is still known to the manager we iterate its
// declared commands directly; otherwise we fall back to UnregisterAllCommands
// so that stale entries left over from a previous disable/enable cycle are
// also cleaned up.
func (h *AdminHandler) unregisterPluginCommands(pluginID string) {
	if h.stateMgr == nil {
		return
	}
	if p, ok := h.manager.Get(pluginID); ok {
		for _, def := range p.Commands() {
			h.stateMgr.UnregisterCommand(pluginID, def.Name)
		}
		return
	}
	h.stateMgr.UnregisterAllCommands(pluginID)
}

func (h *AdminHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/admin/plugins/upload", h.handleUpload)
	mux.HandleFunc("POST /api/admin/plugins/{id}/install", h.handleInstall)
	mux.HandleFunc("PUT /api/admin/plugins/{id}/config", h.handleUpdateConfig)
	mux.HandleFunc("POST /api/admin/plugins/{id}/config/validate", h.handleValidateConfig)
	mux.HandleFunc("POST /api/admin/plugins/{id}/update", h.handleUpdate)
	mux.HandleFunc("POST /api/admin/plugins/{id}/disable", h.handleDisable)
	mux.HandleFunc("POST /api/admin/plugins/{id}/enable", h.handleEnable)
	mux.HandleFunc("DELETE /api/admin/plugins/{id}", h.handleDelete)
	mux.HandleFunc("GET /api/admin/plugins/{id}", h.handleGetPlugin)
	mux.HandleFunc("GET /api/admin/plugins", h.handleListPlugins)

	mux.HandleFunc("GET /api/admin/plugins/{id}/versions", h.handleListVersions)
	mux.HandleFunc("POST /api/admin/plugins/{id}/versions/{versionId}/rollback", h.handleRollback)
	mux.HandleFunc("DELETE /api/admin/plugins/{id}/versions/{versionId}", h.handleDeleteVersion)

	mux.HandleFunc("GET /api/admin/registry", h.handleRegistryList)
	mux.HandleFunc("GET /api/admin/registry/{id}/versions", h.handleRegistryVersions)
}
