package plugin

import (
	"context"
	"fmt"

	"SuperBotGo/internal/errs"
	"SuperBotGo/internal/model"
)

type RouterRoleChecker interface {
	CheckAccess(ctx context.Context, userID model.GlobalUserID, user *model.GlobalUser, req *model.RoleRequirements) (bool, error)
}

type UpdateRouter struct {
	plugins *Manager
	roles   RouterRoleChecker
}

func NewUpdateRouter(plugins *Manager, roles RouterRoleChecker) *UpdateRouter {
	return &UpdateRouter{
		plugins: plugins,
		roles:   roles,
	}
}

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

	event := model.NewMessengerEvent(req, p.ID())
	resp, err := p.HandleEvent(ctx, event)
	if err != nil {
		return err
	}

	if resp != nil && resp.Error != "" {
		return fmt.Errorf("plugin %q command %q: %s", p.ID(), req.CommandName, resp.Error)
	}

	return nil
}
