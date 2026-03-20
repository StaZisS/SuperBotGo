package hostapi

import (
	"context"
	"net/http"
)

// DBStore provides database access for plugins.
type DBStore interface {
	Query(ctx context.Context, pluginID string, query map[string]interface{}) ([]map[string]interface{}, error)
	Save(ctx context.Context, pluginID string, record map[string]interface{}) error
}

// HTTPClient executes HTTP requests on behalf of a plugin.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// EventBus publishes events for inter-plugin communication.
type EventBus interface {
	Publish(ctx context.Context, topic string, payload []byte) error
}

// PluginRegistry allows calling other plugins.
type PluginRegistry interface {
	CallPlugin(ctx context.Context, target string, method string, params []byte) ([]byte, error)
}

// Dependencies aggregates all external services available to host functions.
type Dependencies struct {
	DB             DBStore
	HTTP           HTTPClient
	Events         EventBus
	PluginRegistry PluginRegistry
}
