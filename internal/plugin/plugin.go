package plugin

import (
	"context"

	"SuperBotGo/internal/plugin/contract"
	"SuperBotGo/internal/state"
	wasmrt "SuperBotGo/internal/wasm/runtime"
)

type Plugin interface {
	ID() string
	Name() string
	Version() string
	Commands() []*state.CommandDefinition
	HandleEvent(ctx context.Context, event contract.Event) (*contract.EventResponse, error)
}

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
