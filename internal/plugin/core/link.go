package core

import (
	"context"

	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/model"
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

func LinkCommand() *state.CommandDefinition {
	return state.NewCommand("link").
		Description("Account Linking").
		Step("action", func(s *state.StepBuilder) {
			s.Prompt(func(p *state.PromptBuilder) {
				p.LocalizedText("link.title", model.StylePlain)
				p.LocalizedText("link.description", model.StylePlain)
				p.LocalizedOptions("link.action_prompt", func(o *state.OptionsBuilder) {
					o.LocalizedOption("link.generate", "generate")
					o.LocalizedOption("link.enter", "enter")
				})
			})
		}).
		Step("code", func(s *state.StepBuilder) {
			s.VisibleWhen(func(params model.OptionMap) bool {
				return params.Get("action") != "generate"
			})
			s.Prompt(func(p *state.PromptBuilder) {
				p.LocalizedText("link.enter_code_title", model.StylePlain)
				p.LocalizedText("link.enter_code_hint", model.StylePlain)
			})
		}).
		Build()
}

func (p *Plugin) handleLink(ctx context.Context, m *model.MessengerTriggerData) error {
	locale := m.Locale
	action := m.Params.Get("action")
	if action == "" {
		return p.api.Reply(ctx, m, model.NewTextMessage(i18n.Get("link.action_required", locale)))
	}

	var result LinkResult
	switch action {
	case "generate":
		result = p.linker.InitiateLinking(ctx, m.UserID)
	case "enter":
		code := m.Params.Get("code")
		if code == "" {
			return p.api.Reply(ctx, m, model.NewTextMessage(i18n.Get("link.code_required", locale)))
		}
		result = p.linker.CompleteLinking(ctx, m.UserID, code)
	default:
		return p.api.Reply(ctx, m, model.NewTextMessage("Unknown action: "+action))
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

	return p.api.Reply(ctx, m, msg)
}
