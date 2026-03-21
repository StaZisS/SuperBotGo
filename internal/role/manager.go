package role

import (
	"context"
	"log/slog"

	"SuperBotGo/internal/model"
)

type Manager struct {
	store  Store
	logger *slog.Logger
}

func NewManager(store Store, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{store: store, logger: logger}
}

func (m *Manager) AssignRole(ctx context.Context, userID model.GlobalUserID, roleType model.RoleLayer, roleName string) error {
	err := m.store.AddRole(ctx, model.UserRole{
		UserID:   userID,
		RoleType: roleType,
		RoleName: roleName,
	})
	if err != nil {
		return err
	}
	m.logger.Info("role assigned",
		slog.Int64("user_id", int64(userID)),
		slog.String("role_type", string(roleType)),
		slog.String("role_name", roleName))
	return nil
}

func (m *Manager) RevokeRole(ctx context.Context, userID model.GlobalUserID, roleType model.RoleLayer, roleName string) error {
	err := m.store.RemoveRole(ctx, userID, roleType, roleName)
	if err != nil {
		return err
	}
	m.logger.Info("role revoked",
		slog.Int64("user_id", int64(userID)),
		slog.String("role_type", string(roleType)),
		slog.String("role_name", roleName))
	return nil
}

func (m *Manager) SetSystemRole(ctx context.Context, userID model.GlobalUserID, role string) error {
	if err := m.store.ClearRoles(ctx, userID, model.RoleLayerSystem); err != nil {
		return err
	}
	return m.AssignRole(ctx, userID, model.RoleLayerSystem, role)
}
