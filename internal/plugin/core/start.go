package core

import (
	"context"

	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/state"
)

func StartCommand() *state.CommandDefinition {
	return state.NewCommand("start").
		LocalizedDescription(map[string]string{
			"en": "Welcome message",
			"ru": "Приветствие",
		}).
		Description("Welcome message").
		Build()
}

func (p *Plugin) handleStart(ctx context.Context, m *model.MessengerTriggerData) error {
	return p.api.Reply(ctx, m, model.Message{
		Blocks: []model.ContentBlock{
			model.TextBlock{
				Text:  i18n.Get("start.welcome", m.Locale),
				Style: model.StyleHeader,
			},
			model.TextBlock{
				Text:  i18n.Get("start.description", m.Locale),
				Style: model.StylePlain,
			},
			model.OptionsBlock{
				Options: []model.Option{
					{Label: i18n.Get("start.browse_plugins", m.Locale), Value: "/plugins"},
				},
			},
		},
	})
}
