package settings

import (
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/state"
)

func SettingsCommand() *state.CommandDefinition {
	return state.NewCommand("settings").
		Description("User Settings").
		Step("action", func(s *state.StepBuilder) {
			s.Prompt(func(p *state.PromptBuilder) {
				p.LocalizedText("settings.title", model.StyleHeader)
				p.LocalizedOptions("settings.action_prompt", func(o *state.OptionsBuilder) {
					o.LocalizedOption("settings.change_language", "change_language")
					o.LocalizedOption("settings.notification_channel", "notification_channel")
					o.LocalizedOption("settings.mute_mentions", "mute_mentions")
					o.LocalizedOption("settings.work_hours", "work_hours")
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
			b.Case("notification_channel", func(n *state.NodeListBuilder) {
				n.Step("preferred_channel", func(s *state.StepBuilder) {
					s.Prompt(func(p *state.PromptBuilder) {
						p.LocalizedText("settings.select_channel", model.StylePlain)
						p.LocalizedOptions("settings.channel_prompt", func(o *state.OptionsBuilder) {
							o.Add("Telegram", "TELEGRAM")
							o.Add("Discord", "DISCORD")
							o.Add("Telegram → Discord", "TELEGRAM,DISCORD")
							o.Add("Discord → Telegram", "DISCORD,TELEGRAM")
						})
					})
				})
			})
			b.Case("mute_mentions", func(n *state.NodeListBuilder) {
				n.Step("mute_value", func(s *state.StepBuilder) {
					s.Prompt(func(p *state.PromptBuilder) {
						p.LocalizedText("settings.mute_mentions_prompt", model.StylePlain)
						p.LocalizedOptions("settings.mute_mentions_options", func(o *state.OptionsBuilder) {
							o.LocalizedOption("settings.mute_on", "true")
							o.LocalizedOption("settings.mute_off", "false")
						})
					})
				})
			})
			b.Case("work_hours", func(n *state.NodeListBuilder) {
				n.Step("work_hours_value", func(s *state.StepBuilder) {
					s.Prompt(func(p *state.PromptBuilder) {
						p.LocalizedText("settings.work_hours_prompt", model.StylePlain)
						p.LocalizedOptions("settings.work_hours_options", func(o *state.OptionsBuilder) {
							o.LocalizedOption("settings.wh_9_18", "9-18")
							o.LocalizedOption("settings.wh_8_17", "8-17")
							o.LocalizedOption("settings.wh_10_19", "10-19")
							o.LocalizedOption("settings.wh_disable", "off")
						})
					})
				})
				n.Step("timezone", func(s *state.StepBuilder) {
					s.Prompt(func(p *state.PromptBuilder) {
						p.LocalizedText("settings.timezone_prompt", model.StylePlain)
						p.LocalizedOptions("settings.timezone_options", func(o *state.OptionsBuilder) {
							o.Add("Europe/Moscow (UTC+3)", "Europe/Moscow")
							o.Add("Europe/London (UTC+0)", "Europe/London")
							o.Add("Asia/Novosibirsk (UTC+7)", "Asia/Novosibirsk")
							o.Add("Asia/Vladivostok (UTC+10)", "Asia/Vladivostok")
						})
					})
				})
			})
		}).
		Build()
}
