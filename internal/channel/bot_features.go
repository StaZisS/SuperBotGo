package channel

import (
	"net/http"

	"SuperBotGo/internal/model"
)

// CommandRegistrar is an optional interface for bots that expose a platform
// command registration step.
type CommandRegistrar interface {
	RegisterCommands(commands []string)
}

// RouteRegistrar is an optional interface for bots that need to mount public
// HTTP routes before the server starts listening.
type RouteRegistrar interface {
	RegisterRoutes(mux *http.ServeMux) error
}

// OptionLabel resolves the user-facing label for an option with a safe
// fallback to its value.
func OptionLabel(opt model.Option) string {
	if opt.Label != "" {
		return opt.Label
	}
	return opt.Value
}
