package link

import (
	"context"
	"fmt"

	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/state"
)

type LinkResult struct {
	Kind    LinkResultKind
	Code    string
	Message string
}

type LinkResultKind int

const (
	LinkCodeGenerated LinkResultKind = iota
	LinkLinked
	LinkError
)

type AccountLinker interface {
	InitiateLinking(ctx context.Context, userID model.GlobalUserID) LinkResult
	CompleteLinking(ctx context.Context, userID model.GlobalUserID, code string) LinkResult
}

type Plugin struct {
	api    *plugin.SenderAPI
	linker AccountLinker
	cmdDef *state.CommandDefinition
}

func New(api *plugin.SenderAPI, linker AccountLinker) *Plugin {
	return &Plugin{
		api:    api,
		linker: linker,
		cmdDef: LinkCommand(),
	}
}

func (p *Plugin) ID() string                           { return "link" }
func (p *Plugin) Name() string                         { return "Account Linking" }
func (p *Plugin) Version() string                      { return "1.0.0" }
func (p *Plugin) SupportedRoles() []string             { return []string{"USER", "ADMIN"} }
func (p *Plugin) Commands() []*state.CommandDefinition { return []*state.CommandDefinition{p.cmdDef} }

func (p *Plugin) HandleEvent(ctx context.Context, event model.Event) (*model.EventResponse, error) {
	m, err := event.Messenger()
	if err != nil {
		return nil, fmt.Errorf("link: parse messenger data: %w", err)
	}

	locale := m.Locale
	action := m.Params.Get("action")
	if action == "" {
		return nil, p.api.Reply(ctx, m, model.NewTextMessage(i18n.Get("link.action_required", locale)))
	}

	var result LinkResult
	switch action {
	case "generate":
		result = p.linker.InitiateLinking(ctx, m.UserID)
	case "enter":
		code := m.Params.Get("code")
		if code == "" {
			return nil, p.api.Reply(ctx, m, model.NewTextMessage(i18n.Get("link.code_required", locale)))
		}
		result = p.linker.CompleteLinking(ctx, m.UserID, code)
	default:
		return nil, p.api.Reply(ctx, m, model.NewTextMessage("Unknown action: "+action))
	}

	var msg model.Message
	switch result.Kind {
	case LinkCodeGenerated:
		msg = model.Message{
			Blocks: []model.ContentBlock{
				model.TextBlock{Text: i18n.Get("link.your_code", locale), Style: model.StyleHeader},
				model.TextBlock{Text: result.Code, Style: model.StyleCode},
				model.TextBlock{Text: i18n.Get("link.code_expires", locale), Style: model.StylePlain},
			},
		}
	case LinkLinked:
		msg = model.NewTextMessage(result.Message)
	case LinkError:
		msg = model.NewTextMessage(result.Message)
	}

	return nil, p.api.Reply(ctx, m, msg)
}
