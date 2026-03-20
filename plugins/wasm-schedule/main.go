package main

import "github.com/superbot/wasmplugin"

func main() {
	wasmplugin.Run(wasmplugin.Plugin{
		ID:      "schedule",
		Name:    "University Schedule",
		Version: "1.2.0",
		Config: wasmplugin.ConfigFields(
			wasmplugin.String("greeting", "Message shown before the schedule").Default("Welcome! Here is your schedule:"),
			wasmplugin.String("university_name", "University name shown in the header").Default("University"),
		),
		Commands: []wasmplugin.Command{
			{
				Name:        "schedule",
				Description: "Show today's university schedule",
				Steps: []wasmplugin.Step{
					{
						Param:  "building",
						Prompt: "Select building:",
						Options: []wasmplugin.Option{
							{Label: "Building 1", Value: "1"},
							{Label: "Building 2", Value: "2"},
							{Label: "Building 3", Value: "3"},
						},
					},
					{
						Param:      "room",
						Prompt:     "Building {building} — enter room number:",
						Validation: `^\d+$`,
					},
					{
						Param:      "date",
						Prompt:     "Building {building}, room {room} — enter date (YYYY-MM-DD):",
						Validation: `^\d{4}-\d{2}-\d{2}$`,
					},
				},
				Handler: handleSchedule,
			},
		},
	})
}

func handleSchedule(ctx *wasmplugin.CommandContext) error {
	building := ctx.Params["building"]
	room := ctx.Params["room"]
	date := ctx.Params["date"]

	ctx.Log("schedule: building=" + building + " room=" + room + " date=" + date)

	greeting := ctx.Config("greeting", "")
	uniName := ctx.Config("university_name", "")

	var text string
	if greeting != "" {
		text = greeting + "\n\n"
	}
	if uniName != "" {
		text += uniName + "\n"
	}
	text += generateScheduleForBuilding(building, room, date, ctx.Locale)

	ctx.Reply(text)
	return nil
}
