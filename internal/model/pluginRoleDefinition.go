package model

type PluginRoleDefinition struct {
	ID           int64  `json:"id"`
	AssignableBy string `json:"assignable_by"`
	DisplayName  string `json:"display_name"`
	RoleName     string `json:"role_name"`
}
