package core

import (
	"context"

	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/state"
)

// TsuAuthLinker generates a TSU.Accounts authentication URL for account linking.
type TsuAuthLinker interface {
	GenerateAuthURL(userID model.GlobalUserID) (string, error)
}

func LinkCommand() *state.CommandDefinition {
	return state.NewCommand("link").
		Description("Account Linking").
		Build()
}

func (p *Plugin) handleLink(ctx context.Context, m *model.MessengerTriggerData) error {
	locale := m.Locale
	if p.tsuLinker == nil {
		return p.api.Reply(ctx, m, model.NewTextMessage(i18n.Get("link.tsu_unavailable", locale)))
	}

	authURL, err := p.tsuLinker.GenerateAuthURL(m.UserID)
	if err != nil {
		return p.api.Reply(ctx, m, model.NewTextMessage(i18n.Get("link.tsu_error", locale)))
	}

	msg := model.Message{
		Blocks: []model.ContentBlock{
			model.TextBlock{Text: i18n.Get("link.tsu_title", locale), Style: model.StyleHeader},
			model.TextBlock{Text: i18n.Get("link.tsu_hint", locale), Style: model.StylePlain},
			model.LinkBlock{URL: authURL, Label: i18n.Get("link.tsu_login_button", locale)},
			model.TextBlock{Text: i18n.Get("link.tsu_expires", locale), Style: model.StylePlain},
		},
	}
	return p.api.Reply(ctx, m, msg)
}
