package trigger

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	wasmrt "SuperBotGo/internal/wasm/runtime"
)

type httpRoute struct {
	PluginID    string
	TriggerName string
	Methods     map[string]bool
}

type Registry struct {
	mu            sync.RWMutex
	httpRoutes    map[string]httpRoute
	apiKeys       map[string]string
	cronScheduler *CronScheduler
}

func NewRegistry() *Registry {
	return &Registry{
		httpRoutes: make(map[string]httpRoute),
		apiKeys:    make(map[string]string),
	}
}

func (r *Registry) SetCronScheduler(cs *CronScheduler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cronScheduler = cs
}

func httpRouteKey(pluginID, path string) string {
	path = strings.TrimPrefix(path, "/")
	return pluginID + "/" + path
}

func (r *Registry) RegisterTriggers(pluginID string, triggers []wasmrt.TriggerDef) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, t := range triggers {
		switch t.Type {
		case "http":
			methods := make(map[string]bool, len(t.Methods))
			for _, m := range t.Methods {
				methods[strings.ToUpper(m)] = true
			}
			if len(methods) == 0 {
				methods["GET"] = true
				methods["POST"] = true
			}
			key := httpRouteKey(pluginID, t.Path)
			r.httpRoutes[key] = httpRoute{
				PluginID:    pluginID,
				TriggerName: t.Name,
				Methods:     methods,
			}
		case "cron":
			if r.cronScheduler != nil && t.Schedule != "" {
				if err := r.cronScheduler.AddSchedule(pluginID, t.Name, t.Schedule); err != nil {
					slog.Error("failed to register cron trigger",
						"plugin", pluginID,
						"trigger", t.Name,
						"schedule", t.Schedule,
						"error", err,
					)
				}
			}
		}
	}
}

func (r *Registry) UnregisterTriggers(pluginID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for key, route := range r.httpRoutes {
		if route.PluginID == pluginID {
			delete(r.httpRoutes, key)
		}
	}
	delete(r.apiKeys, pluginID)

	if r.cronScheduler != nil {
		r.cronScheduler.RemoveAll(pluginID)
	}
}

func (r *Registry) LookupHTTP(pluginID, path, method string) (triggerName string, err error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := httpRouteKey(pluginID, path)
	route, ok := r.httpRoutes[key]
	if !ok {
		return "", fmt.Errorf("no HTTP trigger registered for %s/%s", pluginID, path)
	}
	if !route.Methods[strings.ToUpper(method)] {
		return "", fmt.Errorf("method %s not allowed for %s/%s", method, pluginID, path)
	}
	return route.TriggerName, nil
}

func (r *Registry) SetAPIKey(pluginID, apiKey string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.apiKeys[pluginID] = apiKey
}

func (r *Registry) GetAPIKey(pluginID string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.apiKeys[pluginID]
}

func (r *Registry) ListHTTPRoutes() map[string]httpRoute {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]httpRoute, len(r.httpRoutes))
	for k, v := range r.httpRoutes {
		result[k] = v
	}
	return result
}
