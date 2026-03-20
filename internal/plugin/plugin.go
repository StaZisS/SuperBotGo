package plugin

import (
	"context"

	"SuperBotGo/internal/model"
	"SuperBotGo/internal/state"
)

// Plugin is the interface each bot plugin must implement.
type Plugin interface {
	// ID returns a unique plugin identifier.
	ID() string
	// Name returns the human-readable plugin name.
	Name() string
	// Version returns the plugin version string.
	Version() string
	// SupportedRoles lists role names this plugin supports.
	SupportedRoles() []string
	// Commands returns the command definitions provided by this plugin.
	Commands() []*state.CommandDefinition
	// HandleCommand processes a completed command request.
	HandleCommand(ctx context.Context, req model.CommandRequest) error
}

// CommandNames extracts the command name strings from a plugin's definitions.
func CommandNames(p Plugin) []string {
	defs := p.Commands()
	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.Name
	}
	return names
}
