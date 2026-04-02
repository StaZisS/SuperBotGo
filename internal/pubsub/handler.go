package pubsub

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"

	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/state"
	"SuperBotGo/internal/wasm/adapter"
	"SuperBotGo/internal/wasm/hostapi"
)

type PluginData struct {
	WasmKey    string
	ConfigJSON json.RawMessage
}

type PluginFetcher func(ctx context.Context, id string) (*PluginData, error)

type BlobGetter func(ctx context.Context, key string) (io.ReadCloser, error)

type StateManagerRegistrar interface {
	RegisterCommand(pluginID string, def *state.CommandDefinition)
	UnregisterCommand(pluginID, name string)
}

type AdminEventHandler struct {
	fetchPlugin PluginFetcher
	getBlob     BlobGetter
	loader      *adapter.Loader
	manager     *plugin.Manager
	hostAPI     *hostapi.HostAPI
	stateMgr    StateManagerRegistrar
}

func NewAdminEventHandler(
	fetchPlugin PluginFetcher,
	getBlob BlobGetter,
	loader *adapter.Loader,
	manager *plugin.Manager,
	hostAPI *hostapi.HostAPI,
	stateMgr StateManagerRegistrar,
) *AdminEventHandler {
	return &AdminEventHandler{
		fetchPlugin: fetchPlugin,
		getBlob:     getBlob,
		loader:      loader,
		manager:     manager,
		hostAPI:     hostAPI,
		stateMgr:    stateMgr,
	}
}

func (h *AdminEventHandler) Handle(event AdminEvent) {
	ctx := context.Background()
	slog.Info("pubsub: received event", "type", event.Type, "plugin", event.PluginID, "from", event.InstanceID)

	switch event.Type {
	case EventPluginInstalled, EventPluginEnabled:
		h.handleLoad(ctx, event.PluginID)
	case EventPluginUninstalled:
		h.handleUninstall(ctx, event.PluginID)
	case EventPluginDisabled:
		h.handleDisable(ctx, event.PluginID)
	case EventPluginUpdated:
		h.handleUpdate(ctx, event.PluginID)
	case EventConfigChanged:
		h.handleConfigChanged(ctx, event.PluginID)
	default:
		slog.Warn("pubsub: unknown event type", "type", event.Type)
	}
}

func (h *AdminEventHandler) handleLoad(ctx context.Context, pluginID string) {
	data, err := h.fetchPlugin(ctx, pluginID)
	if err != nil {
		slog.Error("pubsub: failed to get plugin record", "id", pluginID, "error", err)
		return
	}

	wasmBytes, err := h.readBlob(ctx, data.WasmKey)
	if err != nil {
		slog.Error("pubsub: failed to read wasm blob", "id", pluginID, "error", err)
		return
	}

	wp, err := h.loader.LoadPluginFromBytes(ctx, wasmBytes, data.ConfigJSON)
	if err != nil {
		slog.Error("pubsub: failed to load plugin", "id", pluginID, "error", err)
		return
	}

	h.manager.Register(wp)
	if h.stateMgr != nil {
		for _, def := range wp.Commands() {
			h.stateMgr.RegisterCommand(pluginID, def)
		}
	}
	slog.Info("pubsub: plugin loaded", "id", pluginID)
}

func (h *AdminEventHandler) handleDisable(ctx context.Context, pluginID string) {
	h.unregisterCommands(pluginID)
	if err := h.loader.UnloadPlugin(ctx, pluginID); err != nil {
		slog.Warn("pubsub: error unloading plugin", "id", pluginID, "error", err)
	}
	h.manager.Remove(pluginID)
	slog.Info("pubsub: plugin disabled", "id", pluginID)
}

func (h *AdminEventHandler) handleUninstall(ctx context.Context, pluginID string) {
	h.unregisterCommands(pluginID)
	if err := h.loader.UnloadPlugin(ctx, pluginID); err != nil {
		slog.Warn("pubsub: error unloading plugin", "id", pluginID, "error", err)
	}
	h.manager.Remove(pluginID)
	slog.Info("pubsub: plugin uninstalled", "id", pluginID)
}

func (h *AdminEventHandler) handleUpdate(ctx context.Context, pluginID string) {
	data, err := h.fetchPlugin(ctx, pluginID)
	if err != nil {
		slog.Error("pubsub: failed to get plugin record for update", "id", pluginID, "error", err)
		return
	}

	wasmBytes, err := h.readBlob(ctx, data.WasmKey)
	if err != nil {
		slog.Error("pubsub: failed to read wasm blob for update", "id", pluginID, "error", err)
		return
	}

	h.unregisterCommands(pluginID)
	h.manager.Remove(pluginID)

	if err := h.loader.ReloadPluginFromBytes(ctx, pluginID, wasmBytes, data.ConfigJSON); err != nil {
		slog.Error("pubsub: failed to reload plugin", "id", pluginID, "error", err)
		return
	}

	if wp, ok := h.loader.GetPlugin(pluginID); ok {
		h.manager.Register(wp)
		if h.stateMgr != nil {
			for _, def := range wp.Commands() {
				h.stateMgr.RegisterCommand(pluginID, def)
			}
		}
	}
	slog.Info("pubsub: plugin updated", "id", pluginID)
}

func (h *AdminEventHandler) handleConfigChanged(ctx context.Context, pluginID string) {
	data, err := h.fetchPlugin(ctx, pluginID)
	if err != nil {
		slog.Error("pubsub: failed to get plugin record for config update", "id", pluginID, "error", err)
		return
	}
	if wp, ok := h.loader.GetPlugin(pluginID); ok {
		wp.SetConfig(data.ConfigJSON)
		slog.Info("pubsub: plugin config updated", "id", pluginID)
	}
}

func (h *AdminEventHandler) unregisterCommands(pluginID string) {
	if h.stateMgr == nil {
		return
	}
	if p, ok := h.manager.Get(pluginID); ok {
		for _, def := range p.Commands() {
			h.stateMgr.UnregisterCommand(pluginID, def.Name)
		}
	}
}

func (h *AdminEventHandler) readBlob(ctx context.Context, key string) ([]byte, error) {
	rc, err := h.getBlob(ctx, key)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
