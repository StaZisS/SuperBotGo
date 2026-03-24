package role

import (
	"context"

	"SuperBotGo/internal/model"
)

type Store interface {
	GetRoles(ctx context.Context, userID model.GlobalUserID, roleType model.RoleLayer) ([]model.UserRole, error)
	GetAllRoles(ctx context.Context, userID model.GlobalUserID) ([]model.UserRole, error)
	AddRole(ctx context.Context, role model.UserRole) error
	RemoveRole(ctx context.Context, userID model.GlobalUserID, roleType model.RoleLayer, roleName string) error
	ClearRoles(ctx context.Context, userID model.GlobalUserID, roleType model.RoleLayer) error
}
