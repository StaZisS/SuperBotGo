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

type eventRoute struct {
	PluginID    string
	TriggerName string
}

type EventSubscription struct {
	PluginID    string
	TriggerName string
}

type Registry struct {
	mu            sync.RWMutex
	httpRoutes    map[string]httpRoute
	eventRoutes   map[string][]eventRoute
	apiKeys       map[string]string
	cronScheduler *CronScheduler
}

func NewRegistry() *Registry {
	return &Registry{
		httpRoutes:  make(map[string]httpRoute),
		eventRoutes: make(map[string][]eventRoute),
		apiKeys:     make(map[string]string),
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
		case "event":
			if t.Topic != "" {
				r.eventRoutes[t.Topic] = append(r.eventRoutes[t.Topic], eventRoute{
					PluginID:    pluginID,
					TriggerName: t.Name,
				})
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
	for topic, routes := range r.eventRoutes {
		filtered := routes[:0]
		for _, route := range routes {
			if route.PluginID != pluginID {
				filtered = append(filtered, route)
			}
		}
		if len(filtered) == 0 {
			delete(r.eventRoutes, topic)
			continue
		}
		r.eventRoutes[topic] = filtered
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

func (r *Registry) GetAPIKey(pluginID string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.apiKeys[pluginID]
}

func (r *Registry) LookupEventSubscribers(topic string) []EventSubscription {
	r.mu.RLock()
	defer r.mu.RUnlock()

	routes := r.eventRoutes[topic]
	if len(routes) == 0 {
		return nil
	}

	out := make([]EventSubscription, 0, len(routes))
	for _, route := range routes {
		out = append(out, EventSubscription{
			PluginID:    route.PluginID,
			TriggerName: route.TriggerName,
		})
	}
	return out
}
