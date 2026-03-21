package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"SuperBotGo/internal/wasm/adapter"
	"SuperBotGo/internal/wasm/hostapi"
)

type PluginPermHandler struct {
	store   PluginStore
	loader  *adapter.Loader
	hostAPI *hostapi.HostAPI
}

func NewPluginPermHandler(store PluginStore, loader *adapter.Loader, hostAPI *hostapi.HostAPI) *PluginPermHandler {
	return &PluginPermHandler{store: store, loader: loader, hostAPI: hostAPI}
}

func (h *PluginPermHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/plugin-permissions", h.handleListAvailable)
	mux.HandleFunc("GET /api/admin/plugins/{id}/plugin-permissions", h.handleGetPluginPerms)
	mux.HandleFunc("PUT /api/admin/plugins/{id}/plugin-permissions", h.handleUpdatePluginPerms)
}

func (h *PluginPermHandler) handleListAvailable(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, hostapi.AllHostPermissions())
}

type declaredPermission struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type callablePlugin struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type pluginPermissionsResponse struct {
	Declared        []declaredPermission     `json:"declared"`
	Granted         []string                 `json:"granted"`
	AllAvailable    []hostapi.PermissionInfo `json:"all_available"`
	CallablePlugins []callablePlugin         `json:"callable_plugins"`
}

func (h *PluginPermHandler) handleGetPluginPerms(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")

	record, err := h.store.GetPlugin(r.Context(), pluginID)
	if err != nil {
		writeError(w, http.StatusNotFound, "plugin not found")
		return
	}

	granted := record.Permissions
	if granted == nil {
		granted = []string{}
	}

	declared := make([]declaredPermission, 0)
	if wp, ok := h.loader.GetPlugin(pluginID); ok {
		meta := wp.Meta()
		declared = make([]declaredPermission, 0, len(meta.Permissions))
		for _, p := range meta.Permissions {
			declared = append(declared, declaredPermission{
				Key:         p.Key,
				Description: p.Description,
				Required:    p.Required,
			})
		}
	}

	allPlugins := h.loader.AllPlugins()
	callable := make([]callablePlugin, 0, len(allPlugins))
	for _, wp := range allPlugins {
		if wp.ID() != pluginID {
			callable = append(callable, callablePlugin{ID: wp.ID(), Name: wp.Name()})
		}
	}

	writeJSON(w, http.StatusOK, pluginPermissionsResponse{
		Declared:        declared,
		Granted:         granted,
		AllAvailable:    hostapi.AllHostPermissions(),
		CallablePlugins: callable,
	})
}

func (h *PluginPermHandler) handleUpdatePluginPerms(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")

	var body struct {
		Permissions []string `json:"permissions"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	for _, p := range body.Permissions {
		if !hostapi.IsKnownPermission(p) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown permission: %q", p))
			return
		}
	}

	if wp, ok := h.loader.GetPlugin(pluginID); ok {
		meta := wp.Meta()
		newSet := make(map[string]bool, len(body.Permissions))
		for _, p := range body.Permissions {
			newSet[p] = true
		}
		for _, decl := range meta.Permissions {
			if decl.Required && !newSet[decl.Key] {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("cannot revoke required permission: %q", decl.Key))
				return
			}
		}
	}

	record, err := h.store.GetPlugin(r.Context(), pluginID)
	if err != nil {
		writeError(w, http.StatusNotFound, "plugin not found")
		return
	}

	sort.Strings(body.Permissions)
	record.Permissions = body.Permissions
	record.UpdatedAt = time.Now()
	if err := h.store.SavePlugin(r.Context(), record); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save permissions")
		return
	}

	h.hostAPI.GrantPermissions(pluginID, body.Permissions)
	h.loader.UpdatePermissions(pluginID, body.Permissions)

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
