package model

type RoleLayer string

const (
	RoleLayerSystem RoleLayer = "SYSTEM"
	RoleLayerGlobal RoleLayer = "GLOBAL"
	RoleLayerPlugin RoleLayer = "PLUGIN"
)

type UserRole struct {
	ID       int64        `json:"id"`
	UserID   GlobalUserID `json:"user_id"`
	RoleType RoleLayer    `json:"role_type"`
	RoleName string       `json:"role_name"`
}

type RoleRequirements struct {
	SystemRole    string   `json:"system_role,omitempty"`
	GlobalRoles   []string `json:"global_roles,omitempty"`
	PluginID      string   `json:"plugin_id,omitempty"`
	PluginRole    string   `json:"plugin_role,omitempty"`
	ScopeEntityID *int64   `json:"scope_entity_id,omitempty"`
}
