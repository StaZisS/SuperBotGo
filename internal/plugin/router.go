package plugin

import (
	"context"
	"fmt"

	"SuperBotGo/internal/errs"
	"SuperBotGo/internal/model"
)

type UpdateRouter struct {
	plugins *Manager
}

func NewUpdateRouter(plugins *Manager) *UpdateRouter {
	return &UpdateRouter{
		plugins: plugins,
	}
}

func (r *UpdateRouter) Route(ctx context.Context, req model.CommandRequest) error {
	p := r.plugins.GetByCommand(req.CommandName)
	if p == nil {
		return errs.NewUserError(errs.ErrCommandNotFound,
			fmt.Sprintf("no plugin found for command: %s", req.CommandName))
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
