package hostapi

import (
	"context"
	"net/http"
)

type DBStore interface {
	Query(ctx context.Context, pluginID string, query map[string]interface{}) ([]map[string]interface{}, error)
	Save(ctx context.Context, pluginID string, record map[string]interface{}) error
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type EventBus interface {
	Publish(ctx context.Context, topic string, payload []byte) error
}

type PluginRegistry interface {
	CallPlugin(ctx context.Context, target string, method string, params []byte) ([]byte, error)
}

type Notifier interface {
	NotifyUser(ctx context.Context, userID int64, text string, priority int) error
	NotifyChat(ctx context.Context, channelType string, chatID string, text string, priority int) error
	NotifyProject(ctx context.Context, projectID int64, text string, priority int) error
}

type Dependencies struct {
	DB             DBStore
	HTTP           HTTPClient
	Events         EventBus
	PluginRegistry PluginRegistry
	Notifier       Notifier
}
