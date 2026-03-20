package settings

import (
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/state"
)

// SettingsCommand builds the command definition for user settings.
func SettingsCommand() *state.CommandDefinition {
	return state.NewCommand("settings").
		Description("User Settings").
		Step("action", func(s *state.StepBuilder) {
			s.Prompt(func(p *state.PromptBuilder) {
				p.LocalizedText("settings.title", model.StyleHeader)
				p.LocalizedOptions("settings.action_prompt", func(o *state.OptionsBuilder) {
					o.LocalizedOption("settings.change_language", "change_language")
				})
			})
		}).
		Branch("action", func(b *state.BranchBuilder) {
			b.Case("change_language", func(n *state.NodeListBuilder) {
				n.Step("language", func(s *state.StepBuilder) {
					s.Prompt(func(p *state.PromptBuilder) {
						p.LocalizedText("settings.select_language", model.StylePlain)
						p.LocalizedOptions("settings.language_prompt", func(o *state.OptionsBuilder) {
							o.Add("English", "en")
							o.Add("\u0420\u0443\u0441\u0441\u043a\u0438\u0439", "ru")
						})
					})
				})
			})
		}).
		Build()
}
