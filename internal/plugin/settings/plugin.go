package settings

import (
	"context"

	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/state"
)

var localeNames = map[string]string{
	"en": "English",
	"ru": "\u0420\u0443\u0441\u0441\u043a\u0438\u0439",
}

// UserLocaleUpdater updates a user's locale preference.
type UserLocaleUpdater interface {
	UpdateLocale(ctx context.Context, userID model.GlobalUserID, locale string) error
}

// Plugin handles the /settings command.
type Plugin struct {
	api         *plugin.SenderAPI
	userService UserLocaleUpdater
	cmdDef      *state.CommandDefinition
}

// New creates a SettingsPlugin.
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

// HandleCommand processes a completed settings command.
func (p *Plugin) HandleCommand(ctx context.Context, req model.CommandRequest) error {
	switch req.Params.Get("action") {
	case "change_language":
		return p.changeLanguage(ctx, req)
	default:
		return p.api.Reply(ctx, req,
			model.NewTextMessage(i18n.Get("settings.unknown_action", req.Locale)))
	}
}

func (p *Plugin) changeLanguage(ctx context.Context, req model.CommandRequest) error {
	newLocale := req.Params.Get("language")
	if newLocale == "" {
		return nil
	}

	if err := p.userService.UpdateLocale(ctx, req.UserID, newLocale); err != nil {
		return err
	}

	displayName := localeNames[newLocale]
	if displayName == "" {
		displayName = newLocale
	}

	return p.api.Reply(ctx, req, model.Message{
		Blocks: []model.ContentBlock{
			model.TextBlock{
				Text:  i18n.Get("settings.language_updated", newLocale, displayName),
				Style: model.StyleHeader,
			},
		},
	})
}
