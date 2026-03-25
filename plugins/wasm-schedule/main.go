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
			wasmplugin.String("sql_dsn", "PostgreSQL connection string (e.g. postgres://user:pass@host/db)"),
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

// l builds a locale map for English and Russian texts.
func l(en, ru string) map[string]string {
	return map[string]string{"en": en, "ru": ru}
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
		all[i] = wasmplugin.Option{Label: tr(ctx.Locale, "building") + " " + b, Value: b}
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
				LocalizedText(l("University Schedule", "Расписание университета"), wasmplugin.StyleHeader).
				LocalizedDynamicOptions(l("Choose view mode:", "Выберите режим просмотра:"), func(ctx *wasmplugin.CallbackContext) []wasmplugin.Option {
					return []wasmplugin.Option{
						{Label: tr(ctx.Locale, "quick_today"), Value: "quick"},
						{Label: tr(ctx.Locale, "by_date"), Value: "by_date"},
					}
				}),

			wasmplugin.NewStep("building").
				LocalizedText(l("Select building:", "Выберите корпус:"), wasmplugin.StyleSubheader).
				LocalizedPaginatedOptions(l("Building:", "Корпус:"), 2, buildingPages),

			wasmplugin.NewStep("room").
				LocalizedText(l("Enter room number:", "Введите номер аудитории:"), wasmplugin.StylePlain).
				ValidateFunc(func(ctx *wasmplugin.CallbackContext) bool {
					return isDigits(ctx.Input) && len(ctx.Input) <= 4
				}),

			wasmplugin.BranchOn("mode",
				wasmplugin.Case("by_date",
					wasmplugin.NewStep("date").
						LocalizedText(l("Enter date (YYYY-MM-DD):", "Введите дату (ГГГГ-ММ-ДД):"), wasmplugin.StylePlain).
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
		ctx.Reply("Internal error")
		return nil
	}
	defer db.Close()

	entries, err := dbScheduleByBuilding(db, building)
	if err != nil {
		ctx.LogError("db query: " + err.Error())
		ctx.Reply("Internal error")
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
				LocalizedText(l("Search", "Поиск"), wasmplugin.StyleHeader).
				LocalizedText(l("What do you want to find?", "Что вы хотите найти?"), wasmplugin.StylePlain).
				LocalizedDynamicOptions(l("Search by:", "Искать по:"), func(ctx *wasmplugin.CallbackContext) []wasmplugin.Option {
					return []wasmplugin.Option{
						{Label: tr(ctx.Locale, "by_teacher"), Value: "teacher"},
						{Label: tr(ctx.Locale, "by_subject"), Value: "subject"},
						{Label: tr(ctx.Locale, "by_room"), Value: "room"},
					}
				}),

			wasmplugin.BranchOn("what",

				wasmplugin.Case("teacher",
					wasmplugin.NewStep("building").
						LocalizedText(l("Which building?", "Какой корпус?"), wasmplugin.StyleSubheader).
						LocalizedPaginatedOptions(l("Building:", "Корпус:"), 2, buildingPages),

					wasmplugin.NewStep("teacher").
						LocalizedText(l("Select teacher:", "Выберите преподавателя:"), wasmplugin.StylePlain).
						LocalizedDynamicOptions(l("Teacher:", "Преподаватель:"), func(ctx *wasmplugin.CallbackContext) []wasmplugin.Option {
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
						LocalizedText(l("Select subject:", "Выберите предмет:"), wasmplugin.StyleSubheader).
						LocalizedPaginatedOptions(l("Subject:", "Предмет:"), 4, func(ctx *wasmplugin.CallbackContext) wasmplugin.OptionsPage {
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
								opts[i] = wasmplugin.Option{Label: tr(ctx.Locale, s), Value: s}
							}
							return wasmplugin.OptionsPage{
								Options: opts,
								HasMore: end < len(subjects),
							}
						}),
				),

				wasmplugin.Case("room",
					wasmplugin.NewStep("building").
						LocalizedText(l("Which building?", "Какой корпус?"), wasmplugin.StyleSubheader).
						LocalizedPaginatedOptions(l("Building:", "Корпус:"), 2, buildingPages),

					wasmplugin.NewStep("floor").
						LocalizedText(l("Enter floor number (1-9):", "Введите номер этажа (1-9):"), wasmplugin.StylePlain).
						ValidateFunc(func(ctx *wasmplugin.CallbackContext) bool {
							return len(ctx.Input) == 1 && ctx.Input[0] >= '1' && ctx.Input[0] <= '9'
						}),

					wasmplugin.ConditionalBranch(
						wasmplugin.When(
							wasmplugin.ParamEq("building", "3"),
							wasmplugin.NewStep("wing").
								LocalizedText(l("Building 3 — select wing:", "Корпус 3 — выберите крыло:"), wasmplugin.StylePlain).
								LocalizedDynamicOptions(l("Wing:", "Крыло:"), func(ctx *wasmplugin.CallbackContext) []wasmplugin.Option {
									return []wasmplugin.Option{
										{Label: tr(ctx.Locale, "east_wing"), Value: "east"},
										{Label: tr(ctx.Locale, "west_wing"), Value: "west"},
									}
								}),
						),
					),
				),
			),

			wasmplugin.NewStep("notify").
				LocalizedText(l("Enable notifications for this search?", "Включить уведомления для этого поиска?"), wasmplugin.StylePlain).
				LocalizedDynamicOptions(l("Notify:", "Уведомления:"), func(ctx *wasmplugin.CallbackContext) []wasmplugin.Option {
					return []wasmplugin.Option{
						{Label: tr(ctx.Locale, "yes"), Value: "yes"},
						{Label: tr(ctx.Locale, "no"), Value: "no"},
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
