package core

import (
	"context"
	"sort"

	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/locale"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/state"
)

// PluginLister provides a list of user-facing plugins and their commands.
type PluginLister interface {
	ListUserPlugins(excludeIDs ...string) []plugin.PluginInfo
}

// CommandAuthChecker checks whether a user is allowed to execute a command.
type CommandAuthChecker interface {
	CheckCommand(ctx context.Context, userID model.GlobalUserID, pluginID string, commandName string, requirements *model.RoleRequirements) (bool, error)
}

// hiddenCommands are navigational commands that should not appear in plugin command lists.
var hiddenCommands = map[string]struct{}{
	"start":   {},
	"plugins": {},
}

func PluginsCommand(lister PluginLister) *state.CommandDefinition {
	return state.NewCommand("plugins").
		LocalizedDescription(map[string]string{
			"en": "Browse available plugins",
			"ru": "Обзор плагинов",
		}).
		Description("Browse available plugins").
		Step("plugin", func(s *state.StepBuilder) {
			s.Prompt(func(p *state.PromptBuilder) {
				p.LocalizedText("plugins.title", model.StyleHeader)
				p.LocalizedOptions("plugins.choose", func(o *state.OptionsBuilder) {
					o.FromContext(func(ctx state.StepContext) []model.Option {
						plugins := lister.ListUserPlugins()
						sort.Slice(plugins, func(i, j int) bool {
							return plugins[i].Name < plugins[j].Name
						})
						opts := make([]model.Option, 0, len(plugins))
						for _, pl := range plugins {
							// Skip plugins that have no visible commands.
							if countVisibleCommands(pl) == 0 {
								continue
							}
							opts = append(opts, model.Option{Label: pl.Name, Value: pl.ID})
						}
						return opts
					})
				})
			})
		}).
		Build()
}

func countVisibleCommands(p plugin.PluginInfo) int {
	n := 0
	for _, cmd := range p.Commands {
		if _, hidden := hiddenCommands[cmd.Name]; !hidden {
			n++
		}
	}
	return n
}

func (p *Plugin) handlePlugins(ctx context.Context, m *model.MessengerTriggerData) error {
	pluginID := m.Params.Get("plugin")
	if pluginID == "" {
		return p.api.Reply(ctx, m, model.NewTextMessage(i18n.Get("plugins.not_found", m.Locale)))
	}

	plugins := p.pluginLister.ListUserPlugins()
	var info *plugin.PluginInfo
	for i := range plugins {
		if plugins[i].ID == pluginID {
			info = &plugins[i]
			break
		}
	}
	if info == nil {
		return p.api.Reply(ctx, m, model.NewTextMessage(i18n.Get("plugins.not_found", m.Locale)))
	}

	options := make([]model.Option, 0, len(info.Commands)+1)
	for _, cmd := range info.Commands {
		if _, hidden := hiddenCommands[cmd.Name]; hidden {
			continue
		}
		if !p.isCommandAllowed(ctx, m.UserID, info.ID, cmd) {
			continue
		}
		fqName := info.ID + "." + cmd.Name
		label := commandMenuLabel(cmd, m.Locale)
		options = append(options, model.Option{Label: label, Value: "/" + fqName})
	}

	if len(options) == 0 {
		return p.api.Reply(ctx, m, model.Message{
			Blocks: []model.ContentBlock{
				model.TextBlock{Text: info.Name, Style: model.StyleHeader},
				model.TextBlock{Text: i18n.Get("plugins.no_commands", m.Locale), Style: model.StylePlain},
				model.OptionsBlock{
					Options: []model.Option{
						{Label: i18n.Get("plugins.back", m.Locale), Value: "/plugins"},
					},
				},
			},
		})
	}

	// "Back" button
	options = append(options, model.Option{
		Label: i18n.Get("plugins.back", m.Locale),
		Value: "/plugins",
	})

	return p.api.Reply(ctx, m, model.Message{
		Blocks: []model.ContentBlock{
			model.TextBlock{
				Text:  info.Name,
				Style: model.StyleHeader,
			},
			model.OptionsBlock{
				Prompt:  i18n.Get("plugins.commands_prompt", m.Locale),
				Options: options,
			},
		},
	})
}

func (p *Plugin) isCommandAllowed(ctx context.Context, userID model.GlobalUserID, pluginID string, cmd plugin.PluginCommand) bool {
	if p.authChecker == nil {
		return true
	}
	ok, err := p.authChecker.CheckCommand(ctx, userID, pluginID, cmd.Name, cmd.Requirements)
	if err != nil {
		return false
	}
	return ok
}

func commandMenuLabel(cmd plugin.PluginCommand, loc string) string {
	if label := locale.ResolveText(cmd.Descriptions, loc); label != "" {
		return label
	}
	if cmd.Description != "" {
		return cmd.Description
	}
	return cmd.Name
}
