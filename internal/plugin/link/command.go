package link

import (
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/state"
)

// LinkCommand builds the command definition for account linking.
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
