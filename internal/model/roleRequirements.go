package model

type RoleRequirements struct {
	SystemRole    string   `json:"system_role,omitempty"`
	GlobalRoles   []string `json:"global_roles,omitempty"`
	PluginID      string   `json:"plugin_id,omitempty"`
	PluginRole    string   `json:"plugin_role,omitempty"`
	ScopeEntityID *int64   `json:"scope_entity_id,omitempty"`
}
