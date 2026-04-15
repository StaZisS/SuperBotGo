package trigger

import (
	"context"
	"fmt"
	"time"

	"SuperBotGo/internal/metrics"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin"
)

type Router struct {
	registry *Registry
	plugins  *plugin.Manager
	metrics  *metrics.Metrics
}

func NewRouter(registry *Registry, plugins *plugin.Manager) *Router {
	return &Router{
		registry: registry,
		plugins:  plugins,
	}
}

func (r *Router) SetMetrics(m *metrics.Metrics) {
	r.metrics = m
}

func (r *Router) RouteEvent(ctx context.Context, event model.Event) (*model.EventResponse, error) {
	start := time.Now()
	result := "ok"
	defer func() {
		if r.metrics == nil {
			return
		}
		r.metrics.PluginEventHandleDuration.WithLabelValues(
			event.PluginID,
			string(event.TriggerType),
			result,
		).Observe(time.Since(start).Seconds())
	}()

	allPlugins := r.plugins.All()
	p, ok := allPlugins[event.PluginID]
	if !ok {
		result = "dispatch_error"
		return nil, fmt.Errorf("plugin %q not found", event.PluginID)
	}

	resp, err := p.HandleEvent(ctx, event)
	if err != nil {
		result = "dispatch_error"
		return nil, err
	}
	if resp != nil && resp.Error != "" {
		result = "plugin_error"
	}
	return resp, nil
}
