package broadcast

import (
	"fmt"

	"SuperBotGo/internal/model"
	"SuperBotGo/internal/state"
)

// ProjectLister provides the list of projects for the broadcast target selection.
type ProjectLister interface {
	ListProjects() ([]model.Project, error)
}

// BroadcastCommand builds the command definition for broadcasting messages.
func BroadcastCommand(projects ProjectLister) *state.CommandDefinition {
	return state.NewCommand("broadcast").
		Description("Broadcast messages").
		RequireRole("ADMIN", nil).
		Step("target", func(s *state.StepBuilder) {
			s.Prompt(func(p *state.PromptBuilder) {
				p.LocalizedText("broadcast.title", model.StyleHeader)
				p.LocalizedOptions("broadcast.target_prompt", func(o *state.OptionsBuilder) {
					o.LocalizedOption("broadcast.to_user", "to_user")
					o.LocalizedOption("broadcast.to_project", "to_project")
				})
			})
		}).
		Branch("target", func(b *state.BranchBuilder) {
			b.Case("to_user", func(n *state.NodeListBuilder) {
				n.Step("user_id", func(s *state.StepBuilder) {
					s.Validate(func(input model.UserInput) bool {

						text := input.TextValue()
						for _, ch := range text {
							if ch < '0' || ch > '9' {
								return false
							}
						}
						return len(text) > 0
					})
					s.Prompt(func(p *state.PromptBuilder) {
						p.LocalizedText("broadcast.enter_user_id", model.StylePlain)
					})
				})
				n.Step("message_text", func(s *state.StepBuilder) {
					s.Prompt(func(p *state.PromptBuilder) {
						p.LocalizedText("broadcast.enter_message", model.StylePlain)
					})
				})
			})
			b.Case("to_project", func(n *state.NodeListBuilder) {
				n.Step("project_id", func(s *state.StepBuilder) {
					s.Prompt(func(p *state.PromptBuilder) {
						p.LocalizedText("broadcast.select_project", model.StylePlain)
						p.LocalizedOptions("broadcast.project_prompt", func(o *state.OptionsBuilder) {
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
				n.Step("message_text", func(s *state.StepBuilder) {
					s.Prompt(func(p *state.PromptBuilder) {
						p.LocalizedText("broadcast.enter_message", model.StylePlain)
					})
				})
			})
		}).
		Build()
}
