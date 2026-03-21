package resume

import (
	"context"
	"fmt"

	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/state"
)

type DialogReader interface {
	GetCurrentStepMessage(ctx context.Context, userID model.GlobalUserID, locale string) (*model.Message, string, error)
}

type Plugin struct {
	api    *plugin.SenderAPI
	dialog DialogReader
	cmdDef *state.CommandDefinition
}

func New(api *plugin.SenderAPI, dialog DialogReader) *Plugin {
	return &Plugin{
		api:    api,
		dialog: dialog,
		cmdDef: ResumeCommand(),
	}
}

func (p *Plugin) ID() string                           { return "resume" }
func (p *Plugin) Name() string                         { return "Resume Command" }
func (p *Plugin) Version() string                      { return "1.0.0" }
func (p *Plugin) SupportedRoles() []string             { return []string{"USER", "ADMIN"} }
func (p *Plugin) Commands() []*state.CommandDefinition { return []*state.CommandDefinition{p.cmdDef} }

func (p *Plugin) HandleEvent(ctx context.Context, event model.Event) (*model.EventResponse, error) {
	m, err := event.Messenger()
	if err != nil {
		return nil, fmt.Errorf("resume: parse messenger data: %w", err)
	}

	msg, commandName, err := p.dialog.GetCurrentStepMessage(ctx, m.UserID, m.Locale)
	if err != nil {
		return nil, fmt.Errorf("resume: get current step: %w", err)
	}

	if msg == nil {
		return nil, p.api.Reply(ctx, m, model.NewTextMessage(
			i18n.Get("resume.no_active_command", m.Locale)))
	}

	header := model.TextBlock{
		Text:  i18n.Get("resume.continuing", m.Locale, commandName),
		Style: model.StyleSubheader,
	}
	blocks := make([]model.ContentBlock, 0, 1+len(msg.Blocks))
	blocks = append(blocks, header)
	blocks = append(blocks, msg.Blocks...)

	return nil, p.api.Reply(ctx, m, model.Message{Blocks: blocks})
}
