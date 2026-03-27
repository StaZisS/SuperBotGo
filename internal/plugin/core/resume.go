package core

import (
	"context"

	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/state"
)

type DialogReader interface {
	GetCurrentStepMessage(ctx context.Context, userID model.GlobalUserID, locale string) (*model.Message, string, error)
	RelocateDialog(ctx context.Context, userID model.GlobalUserID, chatID string) error
}

func ResumeCommand() *state.CommandDefinition {
	return state.NewCommand("resume").
		Description("Resume active command on this platform").
		PreservesDialog().
		Build()
}

func (p *Plugin) handleResume(ctx context.Context, m *model.MessengerTriggerData) error {
	msg, commandName, err := p.dialog.GetCurrentStepMessage(ctx, m.UserID, m.Locale)
	if err != nil {
		return err
	}

	if msg == nil {
		return p.api.Reply(ctx, m, model.NewTextMessage(
			i18n.Get("resume.no_active_command", m.Locale)))
	}

	if err := p.dialog.RelocateDialog(ctx, m.UserID, m.ChatID); err != nil {
		return err
	}

	header := model.TextBlock{
		Text:  i18n.Get("resume.continuing", m.Locale, commandName),
		Style: model.StyleSubheader,
	}
	blocks := make([]model.ContentBlock, 0, 1+len(msg.Blocks))
	blocks = append(blocks, header)
	blocks = append(blocks, msg.Blocks...)

	return p.api.Reply(ctx, m, model.Message{Blocks: blocks})
}
