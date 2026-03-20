package plugin

import (
	"context"

	"SuperBotGo/internal/model"
	"SuperBotGo/internal/state"
)

type Plugin interface {
	ID() string
	Name() string
	Version() string
	SupportedRoles() []string
	Commands() []*state.CommandDefinition
	HandleCommand(ctx context.Context, req model.CommandRequest) error
}

func CommandNames(p Plugin) []string {
	defs := p.Commands()
	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.Name
	}
	return names
}
