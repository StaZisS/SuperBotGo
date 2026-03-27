package main

import (
	"embed"

	"github.com/superbot/wasmplugin"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
	wasmplugin.Run(wasmplugin.Plugin{
		ID:      "schedule",
		Name:    "University Schedule",
		Version: "1.5.3",
		Config: wasmplugin.ConfigFields(
			wasmplugin.String("greeting", "Message shown before the schedule").Default("Welcome! Here is your schedule:"),
			wasmplugin.String("university_name", "University name shown in the header").Default("University"),
		),
		Requirements: []wasmplugin.Requirement{
			wasmplugin.Database("Store and query schedule entries").Build(),
		},
		Migrations: wasmplugin.MigrationsFromFS(migrationsFS, "migrations"),
		OnConfigure: func(config []byte) error {
			db, err := openDB()
			if err != nil {
				return err
			}
			defer db.Close()
			return seedData(db)
		},
		Triggers: []wasmplugin.Trigger{
			scheduleCommand(),
			findCommand(),
			{
				Name:        "api",
				Type:        wasmplugin.TriggerHTTP,
				Description: "REST API for schedule",
				Path:        "/api/schedule",
				Methods:     []string{"GET"},
				Handler:     handleScheduleHTTP,
			},
			{
				Name:        "daily_reminder",
				Type:        wasmplugin.TriggerCron,
				Description: "Send daily schedule summary every morning",
				Schedule:    "0 7 * * *",
				Handler:     handleDailyReminder,
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

	db, err := openDB()
	if err != nil {
		ctx.JSON(500, map[string]string{"error": "database error"})
		return nil
	}
	defer db.Close()

	entries, err := dbScheduleByBuilding(db, building)
	if err != nil {
		ctx.JSON(500, map[string]string{"error": "query error"})
		return nil
	}

	tr := cat.Tr(locale)

	type entry struct {
		Time    string `json:"time"`
		Subject string `json:"subject"`
		Teacher string `json:"teacher"`
	}

	result := make([]entry, len(entries))
	for i, e := range entries {
		result[i] = entry{
			Time:    e.Time,
			Subject: tr(e.Subject),
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
	db, err := openDB()
	if err != nil {
		return wasmplugin.OptionsPage{}
	}
	defer db.Close()

	buildings, err := dbAllBuildings(db)
	if err != nil {
		return wasmplugin.OptionsPage{}
	}

	all := make([]wasmplugin.Option, len(buildings))
	for i, b := range buildings {
		labels := cat.L("building")
		for loc, text := range labels {
			labels[loc] = text + " " + b
		}
		all[i] = wasmplugin.Option{Label: labels["en"], Labels: labels, Value: b}
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

func scheduleCommand() wasmplugin.Trigger {
	return wasmplugin.Trigger{
		Name:        "schedule",
		Type:        wasmplugin.TriggerMessenger,
		Description: "Show today's university schedule",
		Nodes: []wasmplugin.Node{
			wasmplugin.NewStep("mode").
				LocalizedText(cat.L("schedule"), wasmplugin.StyleHeader).
				LocalizedDynamicOptions(cat.L("choose_mode", "V0", ""), func(ctx *wasmplugin.CallbackContext) []wasmplugin.Option {
					return []wasmplugin.Option{
						cat.Opt("quick_today", "quick"),
						cat.Opt("by_date", "by_date"),
					}
				}),

			wasmplugin.NewStep("building").
				LocalizedText(cat.L("building"), wasmplugin.StyleSubheader).
				LocalizedPaginatedOptions(cat.L("building"), 2, buildingPages),

			wasmplugin.NewStep("room").
				LocalizedText(cat.L("room"), wasmplugin.StylePlain).
				ValidateFunc(func(ctx *wasmplugin.CallbackContext) bool {
					return isDigits(ctx.Input) && len(ctx.Input) <= 4
				}),

			wasmplugin.BranchOn("mode",
				wasmplugin.Case("by_date",
					wasmplugin.NewStep("date").
						LocalizedText(cat.L("enter_date"), wasmplugin.StylePlain).
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

	db, err := openDB()
	if err != nil {
		ctx.LogError("db open: " + err.Error())
		ctx.ReplyLocalized(cat.L("error"))
		return nil
	}
	defer db.Close()

	entries, err := dbScheduleByBuilding(db, building)
	if err != nil {
		ctx.LogError("db query: " + err.Error())
		ctx.ReplyLocalized(cat.L("error"))
		return nil
	}

	greeting := ctx.Config("greeting", "")
	uniName := ctx.Config("university_name", "")

	var text string
	if greeting != "" {
		text = greeting + "\n\n"
	}
	if uniName != "" {
		text += uniName + "\n"
	}
	text += generateScheduleForBuilding(entries, building, room, date, ctx.Locale())

	ctx.Reply(text)
	return nil
}

func findCommand() wasmplugin.Trigger {
	return wasmplugin.Trigger{
		Name:        "find",
		Type:        wasmplugin.TriggerMessenger,
		Description: "Find schedule by teacher, subject, or room",
		Nodes: []wasmplugin.Node{

			wasmplugin.NewStep("what").
				LocalizedText(cat.L("search"), wasmplugin.StyleHeader).
				LocalizedDynamicOptions(cat.L("search"), func(ctx *wasmplugin.CallbackContext) []wasmplugin.Option {
					return []wasmplugin.Option{
						cat.Opt("by_teacher", "teacher"),
						cat.Opt("by_subject", "subject"),
						cat.Opt("by_room", "room"),
					}
				}),

			wasmplugin.BranchOn("what",

				wasmplugin.Case("teacher",
					wasmplugin.NewStep("building").
						LocalizedText(cat.L("building"), wasmplugin.StyleSubheader).
						LocalizedPaginatedOptions(cat.L("building"), 2, buildingPages),

					wasmplugin.NewStep("teacher").
						LocalizedText(cat.L("by_teacher"), wasmplugin.StylePlain).
						LocalizedDynamicOptions(cat.L("by_teacher"), func(ctx *wasmplugin.CallbackContext) []wasmplugin.Option {
							building := ctx.Params["building"]
							db, err := openDB()
							if err != nil {
								return fallbackTeacherOptions(building)
							}
							defer db.Close()
							names, err := dbTeachersByBuilding(db, building)
							if err != nil || len(names) == 0 {
								return fallbackTeacherOptions(building)
							}
							opts := make([]wasmplugin.Option, len(names))
							for i, name := range names {
								opts[i] = wasmplugin.Option{Label: name, Value: name}
							}
							return opts
						}),
				),

				wasmplugin.Case("subject",
					wasmplugin.NewStep("subject").
						LocalizedText(cat.L("by_subject"), wasmplugin.StyleSubheader).
						LocalizedPaginatedOptions(cat.L("by_subject"), 4, func(ctx *wasmplugin.CallbackContext) wasmplugin.OptionsPage {
							db, err := openDB()
							if err != nil {
								return fallbackSubjectPage(ctx)
							}
							defer db.Close()
							subjects, err := dbAllSubjects(db)
							if err != nil || len(subjects) == 0 {
								return fallbackSubjectPage(ctx)
							}
							pageSize := 4
							start := ctx.Page * pageSize
							if start >= len(subjects) {
								return wasmplugin.OptionsPage{}
							}
							end := start + pageSize
							if end > len(subjects) {
								end = len(subjects)
							}
							opts := make([]wasmplugin.Option, end-start)
							for i, s := range subjects[start:end] {
								opts[i] = cat.Opt(s, s)
							}
							return wasmplugin.OptionsPage{
								Options: opts,
								HasMore: end < len(subjects),
							}
						}),
				),

				wasmplugin.Case("room",
					wasmplugin.NewStep("building").
						LocalizedText(cat.L("building"), wasmplugin.StyleSubheader).
						LocalizedPaginatedOptions(cat.L("building"), 2, buildingPages),

					wasmplugin.NewStep("floor").
						LocalizedText(cat.L("find_room_line", "Building", "", "Bld", "", "Floor", "1-9"), wasmplugin.StylePlain).
						ValidateFunc(func(ctx *wasmplugin.CallbackContext) bool {
							return len(ctx.Input) == 1 && ctx.Input[0] >= '1' && ctx.Input[0] <= '9'
						}),

					wasmplugin.ConditionalBranch(
						wasmplugin.When(
							wasmplugin.ParamEq("building", "3"),
							wasmplugin.NewStep("wing").
								LocalizedText(cat.L("east_wing"), wasmplugin.StylePlain).
								LocalizedDynamicOptions(cat.L("east_wing"), func(ctx *wasmplugin.CallbackContext) []wasmplugin.Option {
									return []wasmplugin.Option{
										cat.Opt("east_wing", "east"),
										cat.Opt("west_wing", "west"),
									}
								}),
						),
					),
				),
			),

			wasmplugin.NewStep("notify").
				LocalizedText(cat.L("notify_enabled"), wasmplugin.StylePlain).
				LocalizedDynamicOptions(cat.L("notify_enabled"), func(ctx *wasmplugin.CallbackContext) []wasmplugin.Option {
					return []wasmplugin.Option{
						cat.Opt("yes", "yes"),
						cat.Opt("no", "no"),
					}
				}).
				VisibleWhen(wasmplugin.ParamNeq("what", "room")),
		},

		Handler: func(ctx *wasmplugin.EventContext) error {
			ctx.Log("find: " + ctx.Param("what"))
			ctx.Reply(handleFind(ctx.Locale(), ctx.Messenger.Params))
			return nil
		},
	}
}

func handleDailyReminder(ctx *wasmplugin.EventContext) error {
	ctx.Log("cron: daily_reminder fired")

	db, err := openDB()
	if err != nil {
		ctx.LogError("db open: " + err.Error())
		return nil
	}
	defer db.Close()

	greeting := ctx.Config("greeting", "")
	uniName := ctx.Config("university_name", "")

	var text string
	if greeting != "" {
		text = greeting + "\n\n"
	}
	if uniName != "" {
		text += uniName + "\n"
	}
	text += "Daily schedule summary:\n\n"

	buildings, err := dbAllBuildings(db)
	if err != nil {
		ctx.LogError("db query: " + err.Error())
		ctx.Reply(text + "Failed to load schedule.")
		return nil
	}

	for _, bld := range buildings {
		entries, err := dbScheduleByBuilding(db, bld)
		if err != nil || len(entries) == 0 {
			continue
		}
		text += "Building " + bld + ":\n"
		for _, e := range entries {
			text += "  " + e.Time + "  " + e.Subject + " (" + e.Teacher + ")\n"
		}
		text += "\n"
	}

	ctx.Reply(text)
	return nil
}
