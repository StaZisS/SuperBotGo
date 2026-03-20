package schedule

import (
	"regexp"
	"unicode"

	"SuperBotGo/internal/model"
	"SuperBotGo/internal/state"
)

var datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func ScheduleCommand() *state.CommandDefinition {
	return state.NewCommand("schedule").
		Description("University Schedule").
		Step("building", func(s *state.StepBuilder) {
			s.Prompt(func(p *state.PromptBuilder) {
				p.LocalizedText("schedule.select_building", model.StylePlain)
				p.LocalizedText("schedule.choose_building", model.StylePlain)
				p.LocalizedOptions("schedule.building_prompt", func(o *state.OptionsBuilder) {
					o.LocalizedOption("schedule.building_option", "1")
					o.LocalizedOption("schedule.building_option", "2")
					o.LocalizedOption("schedule.building_option", "3")
				})
			})
		}).
		Step("room", func(s *state.StepBuilder) {
			s.Validate(func(input model.UserInput) bool {
				text := input.TextValue()
				if text == "" {
					return false
				}
				for _, ch := range text {
					if !unicode.IsDigit(ch) {
						return false
					}
				}
				return true
			})
			s.Prompt(func(p *state.PromptBuilder) {
				p.LocalizedText("schedule.enter_room", model.StylePlain)
				p.LocalizedText("schedule.enter_room_hint", model.StylePlain)
			})
		}).
		Step("date", func(s *state.StepBuilder) {
			s.Validate(func(input model.UserInput) bool {
				return datePattern.MatchString(input.TextValue())
			})
			s.Prompt(func(p *state.PromptBuilder) {
				p.LocalizedText("schedule.enter_date", model.StylePlain)
				p.LocalizedText("schedule.enter_date_hint", model.StylePlain)
			})
		}).
		Build()
}
