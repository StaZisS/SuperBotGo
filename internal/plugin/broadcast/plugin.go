package broadcast

import (
	"context"
	"fmt"
	"strconv"

	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/state"
)

type ProjectFinder interface {
	ProjectLister
	FindProject(ctx context.Context, id int64) (*model.Project, error)
}

type Plugin struct {
	api      *plugin.SenderAPI
	projects ProjectFinder
	cmdDef   *state.CommandDefinition
}

func New(api *plugin.SenderAPI, projects ProjectFinder) *Plugin {
	return &Plugin{
		api:      api,
		projects: projects,
		cmdDef:   BroadcastCommand(projects),
	}
}

func (p *Plugin) ID() string                           { return "broadcast" }
func (p *Plugin) Name() string                         { return "Broadcast Messages" }
func (p *Plugin) Version() string                      { return "1.0.0" }
func (p *Plugin) SupportedRoles() []string             { return []string{"ADMIN"} }
func (p *Plugin) Commands() []*state.CommandDefinition { return []*state.CommandDefinition{p.cmdDef} }

func (p *Plugin) HandleCommand(ctx context.Context, req model.CommandRequest) error {
	locale := req.Locale
	target := req.Params.Get("target")
	if target == "" {
		return p.api.Reply(ctx, req, model.NewTextMessage(i18n.Get("broadcast.target_required", locale)))
	}

	messageText := req.Params.Get("message_text")
	if messageText == "" {
		return p.api.Reply(ctx, req, model.NewTextMessage(i18n.Get("broadcast.message_required", locale)))
	}

	broadcastMsg := model.NewTextMessage(messageText)

	switch target {
	case "to_user":
		return p.sendToUser(ctx, req, broadcastMsg)
	case "to_project":
		return p.sendToProject(ctx, req, broadcastMsg)
	default:
		return p.api.Reply(ctx, req, model.NewTextMessage(
			i18n.Get("broadcast.unknown_target", locale, target)))
	}
}

func (p *Plugin) sendToUser(ctx context.Context, req model.CommandRequest, msg model.Message) error {
	locale := req.Locale
	userID, err := strconv.ParseInt(req.Params.Get("user_id"), 10, 64)
	if err != nil {
		return p.api.Reply(ctx, req, model.NewTextMessage(i18n.Get("broadcast.invalid_user_id", locale)))
	}

	if err := p.api.SendToAllChannels(ctx, model.GlobalUserID(userID), msg); err != nil {
		return p.api.Reply(ctx, req, model.NewTextMessage(
			i18n.Get("broadcast.user_not_found", locale, fmt.Sprintf("%d", userID))))
	}

	return p.api.Reply(ctx, req, model.Message{
		Blocks: []model.ContentBlock{
			model.TextBlock{Text: i18n.Get("broadcast.sent", locale), Style: model.StyleHeader},
			model.TextBlock{Text: i18n.Get("broadcast.sent_to_user", locale, fmt.Sprintf("%d", userID)), Style: model.StylePlain},
		},
	})
}

func (p *Plugin) sendToProject(ctx context.Context, req model.CommandRequest, msg model.Message) error {
	locale := req.Locale
	projectID, err := strconv.ParseInt(req.Params.Get("project_id"), 10, 64)
	if err != nil {
		return p.api.Reply(ctx, req, model.NewTextMessage(i18n.Get("broadcast.invalid_project_id", locale)))
	}

	project, err := p.projects.FindProject(ctx, projectID)
	if err != nil || project == nil {
		return p.api.Reply(ctx, req, model.NewTextMessage(i18n.Get("broadcast.project_not_found", locale)))
	}

	if err := p.api.SendToProject(ctx, projectID, msg); err != nil {
		return err
	}

	return p.api.Reply(ctx, req, model.Message{
		Blocks: []model.ContentBlock{
			model.TextBlock{Text: i18n.Get("broadcast.sent", locale), Style: model.StyleHeader},
			model.TextBlock{Text: i18n.Get("broadcast.sent_to_project", locale, project.Name), Style: model.StylePlain},
		},
	})
}
