package api

import (
	"context"
	"encoding/json"
	"time"
)

// PluginRecord is the persistent state of a Wasm plugin.
type PluginRecord struct {
	ID            string          `json:"id"`
	WasmKey       string          `json:"wasm_key"`
	ConfigJSON    json.RawMessage `json:"config_json,omitempty"`
	Permissions   []string        `json:"permissions,omitempty"`
	Enabled       bool            `json:"enabled"`
	SchemaVersion int             `json:"schema_version"`
	WasmHash      string          `json:"wasm_hash"`
	InstalledAt   time.Time       `json:"installed_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// PluginStore persists Wasm plugin state.
type PluginStore interface {
	SavePlugin(ctx context.Context, record PluginRecord) error
	GetPlugin(ctx context.Context, id string) (PluginRecord, error)
	ListPlugins(ctx context.Context) ([]PluginRecord, error)
	DeletePlugin(ctx context.Context, id string) error
}
