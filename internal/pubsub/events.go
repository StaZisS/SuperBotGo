package pubsub

const (
	EventPluginInstalled   = "plugin_installed"
	EventPluginUninstalled = "plugin_uninstalled"
	EventPluginEnabled     = "plugin_enabled"
	EventPluginDisabled    = "plugin_disabled"
	EventPluginUpdated     = "plugin_updated"
	EventConfigChanged     = "config_changed"
	EventPermChanged       = "permissions_changed"
)

// AdminEvent is a lightweight notification sent via PG NOTIFY.
// It carries only the event type and plugin ID; the receiver
// reads full data from the shared PostgreSQL store.
type AdminEvent struct {
	Type       string `json:"type"`
	PluginID   string `json:"plugin_id"`
	InstanceID string `json:"instance_id"`
}
