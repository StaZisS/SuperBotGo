package settings

import (
	"context"
	"fmt"

	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/state"
)

var localeNames = map[string]string{
	"en": "English",
	"ru": "Русский",
}

type UserLocaleUpdater interface {
	UpdateLocale(ctx context.Context, userID model.GlobalUserID, locale string) error
}

type Plugin struct {
	api         *plugin.SenderAPI
	userService UserLocaleUpdater
	cmdDef      *state.CommandDefinition
}

func New(api *plugin.SenderAPI, userService UserLocaleUpdater) *Plugin {
	return &Plugin{
		api:         api,
		userService: userService,
		cmdDef:      SettingsCommand(),
	}
}

func (p *Plugin) ID() string                           { return "settings" }
func (p *Plugin) Name() string                         { return "User Settings" }
func (p *Plugin) Version() string                      { return "1.0.0" }
func (p *Plugin) SupportedRoles() []string             { return []string{"USER", "ADMIN"} }
func (p *Plugin) Commands() []*state.CommandDefinition { return []*state.CommandDefinition{p.cmdDef} }

func (p *Plugin) HandleEvent(ctx context.Context, event model.Event) (*model.EventResponse, error) {
	m, err := event.Messenger()
	if err != nil {
		return nil, fmt.Errorf("settings: parse messenger data: %w", err)
	}

	switch m.Params.Get("action") {
	case "change_language":
		return nil, p.changeLanguage(ctx, m)
	default:
		return nil, p.api.Reply(ctx, m,
			model.NewTextMessage(i18n.Get("settings.unknown_action", m.Locale)))
	}
}

func (p *Plugin) changeLanguage(ctx context.Context, m *model.MessengerTriggerData) error {
	newLocale := m.Params.Get("language")
	if newLocale == "" {
		return nil
	}

	if err := p.userService.UpdateLocale(ctx, m.UserID, newLocale); err != nil {
		return err
	}

	displayName := localeNames[newLocale]
	if displayName == "" {
		displayName = newLocale
	}

	return p.api.Reply(ctx, m, model.Message{
		Blocks: []model.ContentBlock{
			model.TextBlock{
				Text:  i18n.Get("settings.language_updated", newLocale, displayName),
				Style: model.StyleHeader,
			},
		},
	})
}
