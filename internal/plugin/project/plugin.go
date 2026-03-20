package project

import (
	"context"
	"fmt"
	"strconv"

	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/state"
)

// ProjectStore provides CRUD operations for projects.
type ProjectStore interface {
	ProjectLister
	FindProject(ctx context.Context, id int64) (*model.Project, error)
	SaveProject(ctx context.Context, name, description string) (*model.Project, error)
}

// ChatStore provides CRUD operations for chat references and bindings.
type ChatStore interface {
	ChatLister
	FindChat(ctx context.Context, channelType model.ChannelType, platformChatID string) (*model.ChatReference, error)
	FindChatByID(ctx context.Context, id int64) (*model.ChatReference, error)
	RegisterChat(ctx context.Context, ref model.ChatReference) (*model.ChatReference, error)
	BindChat(ctx context.Context, projectID, chatRefID int64) error
}

// Plugin handles the /project command.
type Plugin struct {
	api      *plugin.SenderAPI
	projects ProjectStore
	chats    ChatStore
	cmdDef   *state.CommandDefinition
}

// New creates a ProjectPlugin.
func New(api *plugin.SenderAPI, projects ProjectStore, chats ChatStore) *Plugin {
	return &Plugin{
		api:      api,
		projects: projects,
		chats:    chats,
		cmdDef:   ProjectCommand(projects, chats),
	}
}

func (p *Plugin) ID() string                           { return "project" }
func (p *Plugin) Name() string                         { return "Project Management" }
func (p *Plugin) Version() string                      { return "1.0.0" }
func (p *Plugin) SupportedRoles() []string             { return []string{"ADMIN"} }
func (p *Plugin) Commands() []*state.CommandDefinition { return []*state.CommandDefinition{p.cmdDef} }

// HandleCommand processes a completed project command.
func (p *Plugin) HandleCommand(ctx context.Context, req model.CommandRequest) error {
	locale := req.Locale
	switch req.Params.Get("action") {
	case "register_chat":
		return p.registerChat(ctx, req)
	case "create_project":
		return p.createProject(ctx, req)
	case "bind_chat":
		return p.bindChat(ctx, req)
	case "list_projects":
		return p.listProjects(ctx, req)
	default:
		return p.api.Reply(ctx, req, model.NewTextMessage(i18n.Get("project.unknown_action", locale)))
	}
}

func (p *Plugin) registerChat(ctx context.Context, req model.CommandRequest) error {
	locale := req.Locale
	chatKindName := req.Params.Get("chat_kind")
	if chatKindName == "" {
		return p.api.Reply(ctx, req, model.NewTextMessage(i18n.Get("project.chat_kind_required", locale)))
	}
	chatKind := model.ChatKind(chatKindName)

	title := req.Params.Get("chat_title")
	if title == "" {
		return p.api.Reply(ctx, req, model.NewTextMessage(i18n.Get("project.chat_title_required", locale)))
	}

	existing, _ := p.chats.FindChat(ctx, req.ChannelType, req.ChatID)
	if existing != nil {
		return p.api.Reply(ctx, req, model.NewTextMessage(
			i18n.Get("project.chat_already_registered", locale, fmt.Sprintf("%d", existing.ID))))
	}

	chatRef, err := p.chats.RegisterChat(ctx, model.ChatReference{
		ChannelType:    req.ChannelType,
		PlatformChatID: req.ChatID,
		ChatKind:       chatKind,
		Title:          title,
	})
	if err != nil {
		return err
	}

	return p.api.Reply(ctx, req, model.Message{
		Blocks: []model.ContentBlock{
			model.TextBlock{Text: i18n.Get("project.chat_registered", locale), Style: model.StyleHeader},
			model.TextBlock{Text: i18n.Get("project.id_label", locale, fmt.Sprintf("%d", chatRef.ID)), Style: model.StylePlain},
			model.TextBlock{Text: i18n.Get("project.type_label", locale, string(chatRef.ChatKind)), Style: model.StylePlain},
			model.TextBlock{Text: i18n.Get("project.title_label", locale, chatRef.Title), Style: model.StylePlain},
		},
	})
}

func (p *Plugin) createProject(ctx context.Context, req model.CommandRequest) error {
	locale := req.Locale
	name := req.Params.Get("project_name")
	if name == "" {
		return p.api.Reply(ctx, req, model.NewTextMessage(i18n.Get("project.name_required", locale)))
	}
	description := req.Params.Get("project_description")
	if description == "" {
		return p.api.Reply(ctx, req, model.NewTextMessage(i18n.Get("project.description_required", locale)))
	}

	proj, err := p.projects.SaveProject(ctx, name, description)
	if err != nil {
		return err
	}

	return p.api.Reply(ctx, req, model.Message{
		Blocks: []model.ContentBlock{
			model.TextBlock{Text: i18n.Get("project.created", locale), Style: model.StyleHeader},
			model.TextBlock{Text: i18n.Get("project.id_label", locale, fmt.Sprintf("%d", proj.ID)), Style: model.StylePlain},
			model.TextBlock{Text: i18n.Get("project.name_label", locale, proj.Name), Style: model.StylePlain},
			model.TextBlock{Text: i18n.Get("project.description_label", locale, proj.Description), Style: model.StylePlain},
		},
	})
}

func (p *Plugin) bindChat(ctx context.Context, req model.CommandRequest) error {
	locale := req.Locale
	projectID, err1 := strconv.ParseInt(req.Params.Get("project_id"), 10, 64)
	chatRefID, err2 := strconv.ParseInt(req.Params.Get("chat_ref_id"), 10, 64)

	if err1 != nil || err2 != nil {
		return p.api.Reply(ctx, req, model.NewTextMessage(i18n.Get("project.invalid_ids", locale)))
	}

	proj, err := p.projects.FindProject(ctx, projectID)
	if err != nil || proj == nil {
		return p.api.Reply(ctx, req, model.NewTextMessage(i18n.Get("project.not_found", locale)))
	}

	chatRef, err := p.chats.FindChatByID(ctx, chatRefID)
	if err != nil || chatRef == nil {
		return p.api.Reply(ctx, req, model.NewTextMessage(i18n.Get("project.chat_not_found", locale)))
	}

	if err := p.chats.BindChat(ctx, projectID, chatRefID); err != nil {
		return err
	}

	chatTitle := chatRef.Title
	if chatTitle == "" {
		chatTitle = chatRef.PlatformChatID
	}

	return p.api.Reply(ctx, req, model.Message{
		Blocks: []model.ContentBlock{
			model.TextBlock{Text: i18n.Get("project.chat_bound", locale), Style: model.StyleHeader},
			model.TextBlock{Text: i18n.Get("project.bound_project", locale, proj.Name), Style: model.StylePlain},
			model.TextBlock{Text: i18n.Get("project.bound_chat", locale, chatTitle), Style: model.StylePlain},
		},
	})
}

func (p *Plugin) listProjects(ctx context.Context, req model.CommandRequest) error {
	locale := req.Locale
	projs, err := p.projects.ListProjects()
	if err != nil {
		return err
	}
	if len(projs) == 0 {
		return p.api.Reply(ctx, req, model.NewTextMessage(i18n.Get("project.no_projects", locale)))
	}

	blocks := []model.ContentBlock{
		model.TextBlock{Text: i18n.Get("project.projects_header", locale), Style: model.StyleHeader},
	}
	for _, proj := range projs {
		blocks = append(blocks,
			model.TextBlock{Text: "", Style: model.StylePlain},
			model.TextBlock{
				Text:  fmt.Sprintf("#%d %s", proj.ID, proj.Name),
				Style: model.StyleSubheader,
			},
		)
		if proj.Description != "" {
			blocks = append(blocks,
				model.TextBlock{Text: proj.Description, Style: model.StylePlain},
			)
		}
	}

	return p.api.Reply(ctx, req, model.Message{Blocks: blocks})
}
