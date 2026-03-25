package hostapi

import "strings"

// RequirementType describes a type of resource a plugin can require.
type RequirementType struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	HasConfig   bool   `json:"has_config"`
}

// AllRequirementTypes returns the list of supported requirement types.
func AllRequirementTypes() []RequirementType {
	return []RequirementType{
		{Type: "database", Description: "SQL database access (PostgreSQL)", HasConfig: true},
		{Type: "db", Description: "Legacy db_query/db_save host functions", HasConfig: false},
		{Type: "http", Description: "Outbound HTTP requests", HasConfig: false},
		{Type: "kv", Description: "Key-value store", HasConfig: false},
		{Type: "notify", Description: "Send notifications", HasConfig: false},
		{Type: "events", Description: "Publish events", HasConfig: false},
		{Type: "plugin", Description: "Call another plugin", HasConfig: false},
	}
}

// PermissionInfo is kept for backward compatibility with admin API.
type PermissionInfo struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// AllHostPermissions returns flat internal permission keys (used by admin API).
func AllHostPermissions() []PermissionInfo {
	return []PermissionInfo{
		{Key: "db", Description: "Legacy database access (db_query, db_save)", Category: "database"},
		{Key: "sql", Description: "SQL database access", Category: "database"},
		{Key: "network", Description: "Outbound HTTP requests", Category: "network"},
		{Key: "kv", Description: "Key-value store access", Category: "kv"},
		{Key: "notify", Description: "Send notifications", Category: "notifications"},
		{Key: "events", Description: "Publish events", Category: "events"},
	}
}

func IsKnownPermission(key string) bool {
	for _, p := range AllHostPermissions() {
		if p.Key == key {
			return true
		}
	}
	return strings.HasPrefix(key, "plugins:call:") && len(key) > len("plugins:call:")
}
