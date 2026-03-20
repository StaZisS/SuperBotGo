package link

import (
	"context"

	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/state"
)

// LinkResult represents the outcome of a linking operation.
type LinkResult struct {
	Kind    LinkResultKind
	Code    string // for CodeGenerated
	Message string // for Linked or Error
}

// LinkResultKind enumerates the types of link results.
type LinkResultKind int

const (
	LinkCodeGenerated LinkResultKind = iota
	LinkLinked
	LinkError
)

// AccountLinker handles account linking operations.
type AccountLinker interface {
	InitiateLinking(ctx context.Context, userID model.GlobalUserID) LinkResult
	CompleteLinking(ctx context.Context, userID model.GlobalUserID, code string) LinkResult
}

// Plugin handles the /link command.
type Plugin struct {
	api    *plugin.SenderAPI
	linker AccountLinker
	cmdDef *state.CommandDefinition
}

// New creates a LinkPlugin.
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

// HandleCommand processes a completed link command.
func (p *Plugin) HandleCommand(ctx context.Context, req model.CommandRequest) error {
	locale := req.Locale
	action := req.Params.Get("action")
	if action == "" {
		return p.api.Reply(ctx, req, model.NewTextMessage(i18n.Get("link.action_required", locale)))
	}

	var result LinkResult
	switch action {
	case "generate":
		result = p.linker.InitiateLinking(ctx, req.UserID)
	case "enter":
		code := req.Params.Get("code")
		if code == "" {
			return p.api.Reply(ctx, req, model.NewTextMessage(i18n.Get("link.code_required", locale)))
		}
		result = p.linker.CompleteLinking(ctx, req.UserID, code)
	default:
		return p.api.Reply(ctx, req, model.NewTextMessage("Unknown action: "+action))
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

	return p.api.Reply(ctx, req, msg)
}
