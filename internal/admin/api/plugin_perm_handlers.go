package api

import (
	"net/http"

	"SuperBotGo/internal/wasm/adapter"
	"SuperBotGo/internal/wasm/hostapi"
)

type PluginPermHandler struct {
	store   PluginStore
	loader  *adapter.Loader
	hostAPI *hostapi.HostAPI
}

func NewPluginPermHandler(store PluginStore, loader *adapter.Loader, hostAPI *hostapi.HostAPI, bus interface{}) *PluginPermHandler {
	return &PluginPermHandler{store: store, loader: loader, hostAPI: hostAPI}
}

func (h *PluginPermHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/plugin-requirements", h.handleListRequirementTypes)
	mux.HandleFunc("GET /api/admin/plugins/{id}/requirements", h.handleGetPluginRequirements)
}

func (h *PluginPermHandler) handleListRequirementTypes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, hostapi.AllRequirementTypes())
}

type declaredRequirement struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Target      string `json:"target,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

type pluginRequirementsResponse struct {
	Requirements []declaredRequirement `json:"requirements"`
}

func (h *PluginPermHandler) handleGetPluginRequirements(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")

	declared := make([]declaredRequirement, 0)
	if wp, ok := h.loader.GetPlugin(pluginID); ok {
		meta := wp.Meta()
		declared = make([]declaredRequirement, 0, len(meta.Requirements))
		for _, req := range meta.Requirements {
			declared = append(declared, declaredRequirement{
				Type:        req.Type,
				Description: req.Description,
				Target:      req.Target,
				Required:    req.Required,
			})
		}
	}

	writeJSON(w, http.StatusOK, pluginRequirementsResponse{
		Requirements: declared,
	})
}
