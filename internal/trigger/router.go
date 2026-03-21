package trigger

import (
	"context"
	"fmt"

	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin"
)

// Router dispatches events to the correct plugin.
type Router struct {
	registry *Registry
	plugins  *plugin.Manager
}

func NewRouter(registry *Registry, plugins *plugin.Manager) *Router {
	return &Router{
		registry: registry,
		plugins:  plugins,
	}
}

// RouteEvent dispatches an event to the plugin that owns it.
func (r *Router) RouteEvent(ctx context.Context, event model.Event) (*model.EventResponse, error) {
	allPlugins := r.plugins.All()
	p, ok := allPlugins[event.PluginID]
	if !ok {
		return nil, fmt.Errorf("plugin %q not found", event.PluginID)
	}

	return p.HandleEvent(ctx, event)
}
