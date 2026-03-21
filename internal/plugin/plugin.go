package plugin

import (
	"context"

	"SuperBotGo/internal/model"
	"SuperBotGo/internal/state"
	wasmrt "SuperBotGo/internal/wasm/runtime"
)

// Plugin is the core interface every plugin implements.
type Plugin interface {
	ID() string
	Name() string
	Version() string
	SupportedRoles() []string
	Commands() []*state.CommandDefinition
	HandleEvent(ctx context.Context, event model.Event) (*model.EventResponse, error)
}

// TriggerProvider is an optional interface plugins may implement
// to declare non-command triggers (HTTP, cron, events).
type TriggerProvider interface {
	Triggers() []wasmrt.TriggerDef
}

func CommandNames(p Plugin) []string {
	defs := p.Commands()
	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.Name
	}
	return names
}
