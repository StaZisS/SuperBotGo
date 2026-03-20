package project

import (
	"fmt"

	"SuperBotGo/internal/model"
	"SuperBotGo/internal/state"
)

// ProjectLister provides the list of projects for option selection.
type ProjectLister interface {
	ListProjects() ([]model.Project, error)
}

// ChatLister provides the list of chat references for option selection.
type ChatLister interface {
	ListChats() ([]model.ChatReference, error)
}

// ProjectCommand builds the command definition for project management.
func ProjectCommand(projects ProjectLister, chats ChatLister) *state.CommandDefinition {
	return state.NewCommand("project").
		Description("Project management").
		RequireRole("ADMIN", nil).
		Step("action", func(s *state.StepBuilder) {
			s.Prompt(func(p *state.PromptBuilder) {
				p.LocalizedText("project.title", model.StyleHeader)
				p.LocalizedOptions("project.action_prompt", func(o *state.OptionsBuilder) {
					o.LocalizedOption("project.register_chat", "register_chat")
					o.LocalizedOption("project.create_project", "create_project")
					o.LocalizedOption("project.bind_chat", "bind_chat")
					o.LocalizedOption("project.list_projects", "list_projects")
				})
			})
		}).
		Branch("action", func(b *state.BranchBuilder) {
			b.Case("register_chat", func(n *state.NodeListBuilder) {
				n.Step("chat_kind", func(s *state.StepBuilder) {
					s.Prompt(func(p *state.PromptBuilder) {
						p.LocalizedText("project.select_chat_type", model.StylePlain)
						p.LocalizedOptions("project.chat_type_prompt", func(o *state.OptionsBuilder) {
							o.Add("Group", string(model.ChatKindGroup))
							o.Add("Private", string(model.ChatKindPrivate))
							o.Add("Channel", string(model.ChatKindChannel))
						})
					})
				})
				n.Step("chat_title", func(s *state.StepBuilder) {
					s.Prompt(func(p *state.PromptBuilder) {
						p.LocalizedText("project.enter_chat_title", model.StylePlain)
					})
				})
			})
			b.Case("create_project", func(n *state.NodeListBuilder) {
				n.Step("project_name", func(s *state.StepBuilder) {
					s.Prompt(func(p *state.PromptBuilder) {
						p.LocalizedText("project.enter_name", model.StylePlain)
					})
				})
				n.Step("project_description", func(s *state.StepBuilder) {
					s.Prompt(func(p *state.PromptBuilder) {
						p.LocalizedText("project.enter_description", model.StylePlain)
					})
				})
			})
			b.Case("bind_chat", func(n *state.NodeListBuilder) {
				n.Step("project_id", func(s *state.StepBuilder) {
					s.Prompt(func(p *state.PromptBuilder) {
						p.LocalizedText("project.select_project", model.StylePlain)
						p.LocalizedOptions("project.project_prompt", func(o *state.OptionsBuilder) {
							o.From(func() []model.Option {
								projs, err := projects.ListProjects()
								if err != nil {
									return nil
								}
								opts := make([]model.Option, len(projs))
								for i, proj := range projs {
									opts[i] = model.Option{
										Label: proj.Name,
										Value: fmt.Sprintf("%d", proj.ID),
									}
								}
								return opts
							})
						})
					})
				})
				n.Step("chat_ref_id", func(s *state.StepBuilder) {
					s.Prompt(func(p *state.PromptBuilder) {
						p.LocalizedText("project.select_chat", model.StylePlain)
						p.LocalizedOptions("project.chat_prompt", func(o *state.OptionsBuilder) {
							o.From(func() []model.Option {
								chatRefs, err := chats.ListChats()
								if err != nil {
									return nil
								}
								opts := make([]model.Option, len(chatRefs))
								for i, cr := range chatRefs {
									label := cr.PlatformChatID
									if cr.Title != "" {
										label = cr.Title
									}
									label = fmt.Sprintf("%s (%s)", label, cr.ChannelType)
									opts[i] = model.Option{
										Label: label,
										Value: fmt.Sprintf("%d", cr.ID),
									}
								}
								return opts
							})
						})
					})
				})
			})

			b.Case("list_projects", func(n *state.NodeListBuilder) {

			})
		}).
		Build()
}
