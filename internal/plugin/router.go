package plugin

import (
	"context"
	"fmt"

	"SuperBotGo/internal/errs"
	"SuperBotGo/internal/model"
)

// RouterRoleChecker checks roles for the UpdateRouter.
type RouterRoleChecker interface {
	CheckAccess(ctx context.Context, userID model.GlobalUserID, user *model.GlobalUser, req *model.RoleRequirements) (bool, error)
}

// UpdateRouter routes completed CommandRequests to the appropriate plugin.
type UpdateRouter struct {
	plugins *Manager
	roles   RouterRoleChecker
}

// NewUpdateRouter creates a new UpdateRouter.
func NewUpdateRouter(plugins *Manager, roles RouterRoleChecker) *UpdateRouter {
	return &UpdateRouter{
		plugins: plugins,
		roles:   roles,
	}
}

// Route finds the plugin for the command and invokes its HandleCommand method.
func (r *UpdateRouter) Route(ctx context.Context, req model.CommandRequest) error {
	p := r.plugins.GetByCommand(req.CommandName)
	if p == nil {
		return errs.NewUserError(errs.ErrCommandNotFound,
			fmt.Sprintf("no plugin found for command: %s", req.CommandName))
	}

	def := r.plugins.GetCommandDefinition(req.CommandName)
	if def != nil && def.Requirements != nil {
		ok, err := r.roles.CheckAccess(ctx, req.UserID, nil, def.Requirements)
		if err != nil {
			return err
		}
		if !ok {
			return errs.NewUserError(errs.ErrPermissionDenied,
				fmt.Sprintf("insufficient permissions for command '%s'", req.CommandName))
		}
	}

	return p.HandleCommand(ctx, req)
}
