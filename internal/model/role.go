package model

// RoleLayer identifies which layer of the role hierarchy a role belongs to.
type RoleLayer string

const (
	RoleLayerSystem RoleLayer = "SYSTEM"
	RoleLayerGlobal RoleLayer = "GLOBAL"
	RoleLayerPlugin RoleLayer = "PLUGIN"
)

// UserRole associates a user with a named role at a specific layer.
type UserRole struct {
	ID       int64        `json:"id"`
	UserID   GlobalUserID `json:"user_id"`
	RoleType RoleLayer    `json:"role_type"`
	RoleName string       `json:"role_name"`
}

// RoleRequirements describes the roles a user must hold to perform an action.
type RoleRequirements struct {
	SystemRole    string   `json:"system_role,omitempty"`
	GlobalRoles   []string `json:"global_roles,omitempty"`
	PluginID      string   `json:"plugin_id,omitempty"`
	PluginRole    string   `json:"plugin_role,omitempty"`
	ScopeEntityID *int64   `json:"scope_entity_id,omitempty"`
}
