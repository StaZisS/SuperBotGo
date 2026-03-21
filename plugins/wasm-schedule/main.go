package main

import "github.com/superbot/wasmplugin"

func main() {
	wasmplugin.Run(wasmplugin.Plugin{
		ID:      "schedule",
		Name:    "University Schedule",
		Version: "1.4.0",
		Config: wasmplugin.ConfigFields(
			wasmplugin.String("greeting", "Message shown before the schedule").Default("Welcome! Here is your schedule:"),
			wasmplugin.String("university_name", "University name shown in the header").Default("University"),
		),
		Permissions: []wasmplugin.Permission{
			{Key: "triggers:http", Description: "Serve schedule via HTTP API", Required: false},
		},
		Commands: []wasmplugin.Command{
			scheduleCommand(),
			findCommand(),
		},
		Triggers: []wasmplugin.Trigger{
			{
				Name:        "api",
				Type:        wasmplugin.TriggerHTTP,
				Description: "REST API for schedule",
				Path:        "/api/schedule",
				Methods:     []string{"GET"},
				Handler:     handleScheduleHTTP,
			},
		},
	})
}

func handleScheduleHTTP(ctx *wasmplugin.EventContext) error {
	building := ctx.HTTP.Query["building"]
	room := ctx.HTTP.Query["room"]
	date := ctx.HTTP.Query["date"]

	if building == "" {
		ctx.JSON(400, map[string]string{"error": "missing 'building' query parameter"})
		return nil
	}
	if room == "" {
		room = "—"
	}
	if date == "" {
		date = "today"
	}

	locale := ctx.HTTP.Query["locale"]
	if locale == "" {
		locale = "en"
	}

	entries := schedule[building]

	type entry struct {
		Time    string `json:"time"`
		Subject string `json:"subject"`
		Teacher string `json:"teacher"`
	}

	result := make([]entry, len(entries))
	for i, e := range entries {
		result[i] = entry{
			Time:    e.Time,
			Subject: tr(locale, e.Subject),
			Teacher: e.Teacher,
		}
	}

	ctx.JSON(200, map[string]interface{}{
		"building": building,
		"room":     room,
		"date":     date,
		"classes":  result,
	})

	ctx.Log("http: schedule building=" + building + " room=" + room)
	return nil
}

func buildingPages(ctx *wasmplugin.CallbackContext) wasmplugin.OptionsPage {
	all := []wasmplugin.Option{
		{Label: "Building 1", Value: "1"},
		{Label: "Building 2", Value: "2"},
		{Label: "Building 3", Value: "3"},
	}
	pageSize := 2
	start := ctx.Page * pageSize
	if start >= len(all) {
		return wasmplugin.OptionsPage{}
	}
	end := start + pageSize
	if end > len(all) {
		end = len(all)
	}
	return wasmplugin.OptionsPage{
		Options: all[start:end],
		HasMore: end < len(all),
	}
}

func isDigits(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func scheduleCommand() wasmplugin.Command {
	return wasmplugin.Command{
		Name:        "schedule",
		Description: "Show today's university schedule",
		Nodes: []wasmplugin.Node{
			wasmplugin.NewStep("mode").
				Text("University Schedule", wasmplugin.StyleHeader).
				Options("Choose view mode:",
					wasmplugin.Opt("Quick (today)", "quick"),
					wasmplugin.Opt("By date", "by_date"),
				),

			wasmplugin.NewStep("building").
				Text("Select building:", wasmplugin.StyleSubheader).
				PaginatedOptions("Building:", 2, buildingPages),

			wasmplugin.NewStep("room").
				Text("Enter room number:", wasmplugin.StylePlain).
				ValidateFunc(func(ctx *wasmplugin.CallbackContext) bool {
					return isDigits(ctx.Input) && len(ctx.Input) <= 4
				}),

			wasmplugin.BranchOn("mode",
				wasmplugin.Case("by_date",
					wasmplugin.NewStep("date").
						Text("Enter date (YYYY-MM-DD):", wasmplugin.StylePlain).
						Validate(`^\d{4}-\d{2}-\d{2}$`),
				),
			),
		},
		Handler: handleScheduleCmd,
	}
}

func handleScheduleCmd(ctx *wasmplugin.EventContext) error {
	mode := ctx.Param("mode")
	building := ctx.Param("building")
	room := ctx.Param("room")

	date := ctx.Param("date")
	if mode == "quick" {
		date = "today"
	}

	ctx.Log("schedule: mode=" + mode + " building=" + building + " room=" + room + " date=" + date)

	greeting := ctx.Config("greeting", "")
	uniName := ctx.Config("university_name", "")

	var text string
	if greeting != "" {
		text = greeting + "\n\n"
	}
	if uniName != "" {
		text += uniName + "\n"
	}
	text += generateScheduleForBuilding(building, room, date, ctx.Locale())

	ctx.Reply(text)
	return nil
}

func findCommand() wasmplugin.Command {
	return wasmplugin.Command{
		Name:        "find",
		Description: "Find schedule by teacher, subject, or room",
		Nodes: []wasmplugin.Node{

			wasmplugin.NewStep("what").
				Text("Search", wasmplugin.StyleHeader).
				Text("What do you want to find?", wasmplugin.StylePlain).
				Options("Search by:",
					wasmplugin.Opt("By teacher", "teacher"),
					wasmplugin.Opt("By subject", "subject"),
					wasmplugin.Opt("By room", "room"),
				),

			wasmplugin.BranchOn("what",

				wasmplugin.Case("teacher",
					wasmplugin.NewStep("building").
						Text("Which building?", wasmplugin.StyleSubheader).
						PaginatedOptions("Building:", 2, buildingPages),

					wasmplugin.NewStep("teacher").
						Text("Select teacher:", wasmplugin.StylePlain).
						DynamicOptions("Teacher:", func(ctx *wasmplugin.CallbackContext) []wasmplugin.Option {
							names := teachers[ctx.Params["building"]]
							opts := make([]wasmplugin.Option, len(names))
							for i, name := range names {
								opts[i] = wasmplugin.Option{Label: name, Value: name}
							}
							return opts
						}),
				),

				wasmplugin.Case("subject",
					wasmplugin.NewStep("subject").
						Text("Select subject:", wasmplugin.StyleSubheader).
						PaginatedOptions("Subject:", 4, func(ctx *wasmplugin.CallbackContext) wasmplugin.OptionsPage {
							pageSize := 4
							start := ctx.Page * pageSize
							if start >= len(allSubjects) {
								return wasmplugin.OptionsPage{}
							}
							end := start + pageSize
							if end > len(allSubjects) {
								end = len(allSubjects)
							}
							opts := make([]wasmplugin.Option, end-start)
							for i, s := range allSubjects[start:end] {
								opts[i] = wasmplugin.Option{Label: s, Value: s}
							}
							return wasmplugin.OptionsPage{
								Options: opts,
								HasMore: end < len(allSubjects),
							}
						}),
				),

				wasmplugin.Case("room",
					wasmplugin.NewStep("building").
						Text("Which building?", wasmplugin.StyleSubheader).
						PaginatedOptions("Building:", 2, buildingPages),

					wasmplugin.NewStep("floor").
						Text("Enter floor number (1-9):", wasmplugin.StylePlain).
						ValidateFunc(func(ctx *wasmplugin.CallbackContext) bool {
							return len(ctx.Input) == 1 && ctx.Input[0] >= '1' && ctx.Input[0] <= '9'
						}),

					wasmplugin.ConditionalBranch(
						wasmplugin.When(
							wasmplugin.ParamEq("building", "3"),
							wasmplugin.NewStep("wing").
								Text("Building 3 — select wing:", wasmplugin.StylePlain).
								Options("Wing:",
									wasmplugin.Opt("East wing", "east"),
									wasmplugin.Opt("West wing", "west"),
								),
						),
					),
				),
			),

			wasmplugin.NewStep("notify").
				Text("Enable notifications for this search?", wasmplugin.StylePlain).
				Options("Notify:",
					wasmplugin.Opt("Yes", "yes"),
					wasmplugin.Opt("No", "no"),
				).
				VisibleWhen(wasmplugin.ParamNeq("what", "room")),
		},

		Handler: func(ctx *wasmplugin.EventContext) error {
			ctx.Log("find: " + ctx.Param("what"))
			ctx.Reply(handleFind(ctx.Locale(), ctx.Messenger.Params))
			return nil
		},
	}
}
