package broadcast

import (
	"context"
	"fmt"
	"strconv"

	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/notification"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/state"
)

type ProjectFinder interface {
	ProjectLister
	FindProject(ctx context.Context, id int64) (*model.Project, error)
}

type Plugin struct {
	api      *plugin.SenderAPI
	notify   *notification.NotifyAPI
	projects ProjectFinder
	cmdDef   *state.CommandDefinition
}

func New(api *plugin.SenderAPI, notify *notification.NotifyAPI, projects ProjectFinder) *Plugin {
	return &Plugin{
		api:      api,
		notify:   notify,
		projects: projects,
		cmdDef:   BroadcastCommand(projects),
	}
}

func (p *Plugin) ID() string                           { return "broadcast" }
func (p *Plugin) Name() string                         { return "Broadcast Messages" }
func (p *Plugin) Version() string                      { return "1.0.0" }
func (p *Plugin) SupportedRoles() []string             { return []string{"ADMIN"} }
func (p *Plugin) Commands() []*state.CommandDefinition { return []*state.CommandDefinition{p.cmdDef} }

func (p *Plugin) HandleEvent(ctx context.Context, event model.Event) (*model.EventResponse, error) {
	m, err := event.Messenger()
	if err != nil {
		return nil, fmt.Errorf("broadcast: parse messenger data: %w", err)
	}

	locale := m.Locale
	target := m.Params.Get("target")
	if target == "" {
		return nil, p.api.Reply(ctx, m, model.NewTextMessage(i18n.Get("broadcast.target_required", locale)))
	}

	messageText := m.Params.Get("message_text")
	if messageText == "" {
		return nil, p.api.Reply(ctx, m, model.NewTextMessage(i18n.Get("broadcast.message_required", locale)))
	}

	broadcastMsg := model.NewTextMessage(messageText)

	switch target {
	case "to_user":
		return nil, p.sendToUser(ctx, m, broadcastMsg)
	case "to_project":
		return nil, p.sendToProject(ctx, m, broadcastMsg)
	default:
		return nil, p.api.Reply(ctx, m, model.NewTextMessage(
			i18n.Get("broadcast.unknown_target", locale, target)))
	}
}

func (p *Plugin) sendToUser(ctx context.Context, m *model.MessengerTriggerData, msg model.Message) error {
	locale := m.Locale
	userID, err := strconv.ParseInt(m.Params.Get("user_id"), 10, 64)
	if err != nil {
		return p.api.Reply(ctx, m, model.NewTextMessage(i18n.Get("broadcast.invalid_user_id", locale)))
	}

	if err := p.notify.NotifyUser(ctx, model.GlobalUserID(userID), msg, model.PriorityNormal); err != nil {
		return p.api.Reply(ctx, m, model.Message{
			Blocks: []model.ContentBlock{
				model.TextBlock{Text: i18n.Get("broadcast.send_error", locale), Style: model.StyleHeader},
				model.TextBlock{Text: err.Error(), Style: model.StyleCode},
			},
		})
	}

	return p.api.Reply(ctx, m, model.Message{
		Blocks: []model.ContentBlock{
			model.TextBlock{Text: i18n.Get("broadcast.sent", locale), Style: model.StyleHeader},
			model.TextBlock{Text: i18n.Get("broadcast.sent_to_user", locale, fmt.Sprintf("%d", userID)), Style: model.StylePlain},
		},
	})
}

func (p *Plugin) sendToProject(ctx context.Context, m *model.MessengerTriggerData, msg model.Message) error {
	locale := m.Locale
	projectID, err := strconv.ParseInt(m.Params.Get("project_id"), 10, 64)
	if err != nil {
		return p.api.Reply(ctx, m, model.NewTextMessage(i18n.Get("broadcast.invalid_project_id", locale)))
	}

	project, err := p.projects.FindProject(ctx, projectID)
	if err != nil || project == nil {
		return p.api.Reply(ctx, m, model.NewTextMessage(i18n.Get("broadcast.project_not_found", locale)))
	}

	if err := p.notify.NotifyProject(ctx, projectID, msg, model.PriorityNormal); err != nil {
		return p.api.Reply(ctx, m, model.Message{
			Blocks: []model.ContentBlock{
				model.TextBlock{Text: i18n.Get("broadcast.send_error", locale), Style: model.StyleHeader},
				model.TextBlock{Text: err.Error(), Style: model.StyleCode},
			},
		})
	}

	return p.api.Reply(ctx, m, model.Message{
		Blocks: []model.ContentBlock{
			model.TextBlock{Text: i18n.Get("broadcast.sent", locale), Style: model.StyleHeader},
			model.TextBlock{Text: i18n.Get("broadcast.sent_to_project", locale, project.Name), Style: model.StylePlain},
		},
	})
}
