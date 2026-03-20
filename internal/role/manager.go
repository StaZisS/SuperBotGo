package role

import (
	"context"
	"log/slog"

	"SuperBotGo/internal/model"
)

// Manager handles role checking, assignment, and revocation.
type Manager struct {
	store  Store
	logger *slog.Logger
}

// NewManager creates a RoleManager.
func NewManager(store Store, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{store: store, logger: logger}
}

// CheckAccess verifies whether a user has the roles specified in the requirements.
// Returns true if access is granted, false otherwise.
func (m *Manager) CheckAccess(ctx context.Context, userID model.GlobalUserID, _ *model.GlobalUser, req *model.RoleRequirements) (bool, error) {
	if req == nil {
		return true, nil
	}

	if req.SystemRole == "" && len(req.GlobalRoles) == 0 && req.PluginID == "" {
		return true, nil
	}

	if req.SystemRole != "" {
		systemRoles, err := m.store.GetRoles(ctx, userID, model.RoleLayerSystem)
		if err != nil {
			return false, err
		}
		found := false
		for _, r := range systemRoles {
			if r.RoleName == req.SystemRole {
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}

	if len(req.GlobalRoles) > 0 {
		globalRoles, err := m.store.GetRoles(ctx, userID, model.RoleLayerGlobal)
		if err != nil {
			return false, err
		}
		roleSet := make(map[string]bool, len(globalRoles))
		for _, r := range globalRoles {
			roleSet[r.RoleName] = true
		}
		for _, required := range req.GlobalRoles {
			if !roleSet[required] {
				return false, nil
			}
		}
	}

	if req.PluginID != "" && req.PluginRole != "" {
		m.logger.Warn("plugin role check not yet implemented",
			slog.String("plugin_id", req.PluginID),
			slog.String("plugin_role", req.PluginRole))
		return false, nil
	}

	return true, nil
}

// AssignRole adds a role to a user.
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

// RevokeRole removes a specific role from a user.
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

// SetSystemRole clears existing system roles and sets a new one.
func (m *Manager) SetSystemRole(ctx context.Context, userID model.GlobalUserID, role string) error {
	if err := m.store.ClearRoles(ctx, userID, model.RoleLayerSystem); err != nil {
		return err
	}
	return m.AssignRole(ctx, userID, model.RoleLayerSystem, role)
}
