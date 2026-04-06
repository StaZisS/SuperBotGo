package plugin

import "SuperBotGo/internal/model"

// PluginInfo describes a plugin for user-facing display.
type PluginInfo struct {
	ID       string
	Name     string
	Commands []PluginCommand
}

// PluginCommand describes a single command within a plugin.
type PluginCommand struct {
	Name         string
	Description  string
	Requirements *model.RoleRequirements // nil = no restriction
}
