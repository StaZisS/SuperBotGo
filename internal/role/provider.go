package role

import "SuperBotGo/internal/model"

type Provider interface {
	HasRole(userId int64, roleName string, entityId string) bool
	GetUserRoles(userId int64, entityId string) []string
	GetAvailableRoles() []model.PluginRoleDefinition
}
