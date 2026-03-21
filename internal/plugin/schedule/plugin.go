package schedule

import (
	"context"
	"fmt"

	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/state"
)

type scheduleEntry struct {
	Time    string
	Subject string
	Teacher string
}

type Plugin struct {
	api    *plugin.SenderAPI
	cmdDef *state.CommandDefinition
}

func New(api *plugin.SenderAPI) *Plugin {
	return &Plugin{
		api:    api,
		cmdDef: ScheduleCommand(),
	}
}

func (p *Plugin) ID() string                           { return "schedule" }
func (p *Plugin) Name() string                         { return "University Schedule" }
func (p *Plugin) Version() string                      { return "1.0.0" }
func (p *Plugin) SupportedRoles() []string             { return []string{"USER", "ADMIN"} }
func (p *Plugin) Commands() []*state.CommandDefinition { return []*state.CommandDefinition{p.cmdDef} }

func (p *Plugin) HandleEvent(ctx context.Context, event model.Event) (*model.EventResponse, error) {
	m, err := event.Messenger()
	if err != nil {
		return nil, fmt.Errorf("schedule: parse messenger data: %w", err)
	}

	building := m.Params.GetOr("building", "?")
	room := m.Params.GetOr("room", "?")
	date := m.Params.GetOr("date", "?")
	locale := m.Locale

	entries := generateMockSchedule(building, room)

	blocks := []model.ContentBlock{
		model.TextBlock{
			Text:  i18n.Get("schedule.header", locale, building, room, date),
			Style: model.StyleHeader,
		},
		model.TextBlock{Text: "", Style: model.StylePlain},
	}

	for _, entry := range entries {
		blocks = append(blocks, model.TextBlock{
			Text:  entry.Time + "  " + entry.Subject + " (" + entry.Teacher + ")",
			Style: model.StylePlain,
		})
	}

	return nil, p.api.Reply(ctx, m, model.Message{Blocks: blocks})
}

func generateMockSchedule(building, room string) []scheduleEntry {
	mockData := map[string][]scheduleEntry{
		"1": {
			{"08:30-10:00", "Linear Algebra", "Ivanov A.B."},
			{"10:15-11:45", "Programming", "Petrov S.D."},
			{"12:15-13:45", "Physics", "Sidorova E.V."},
			{"14:00-15:30", "English", "Smith J.K."},
		},
		"2": {
			{"08:30-10:00", "Databases", "Kozlov I.P."},
			{"10:15-11:45", "OS", "Morozova T.N."},
			{"12:15-13:45", "Networks", "Volkov D.A."},
		},
		"3": {
			{"10:15-11:45", "Machine Learning", "Kuznetsov R.V."},
			{"12:15-13:45", "Statistics", "Orlova M.S."},
			{"14:00-15:30", "Algorithms", "Novikov P.E."},
			{"15:45-17:15", "Seminar", "Fedorov A.A."},
		},
	}

	if entries, ok := mockData[building]; ok {
		return entries
	}

	return []scheduleEntry{
		{"08:30-10:00", "Lecture Hall " + room, "TBA"},
		{"10:15-11:45", "Seminar " + room, "TBA"},
	}
}
