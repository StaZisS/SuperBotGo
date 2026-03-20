package main

import "github.com/superbot/wasmplugin"

func main() {
	wasmplugin.Run(wasmplugin.Plugin{
		ID:      "schedule",
		Name:    "University Schedule",
		Version: "1.3.0",
		Config: wasmplugin.ConfigFields(
			wasmplugin.String("greeting", "Message shown before the schedule").Default("Welcome! Here is your schedule:"),
			wasmplugin.String("university_name", "University name shown in the header").Default("University"),
		),
		Commands: []wasmplugin.Command{
			scheduleCommand(),
			findCommand(),
		},
	})
}

// buildingPages is a reusable pagination provider for building selection.
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

// isDigits checks that s is non-empty and contains only ASCII digits.
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

// ---------------------------------------------------------------------------
// /schedule command
// ---------------------------------------------------------------------------

func scheduleCommand() wasmplugin.Command {
	return wasmplugin.Command{
		Name:        "schedule",
		Description: "Show today's university schedule",
		Nodes: []wasmplugin.Node{
			// 1. View mode.
			wasmplugin.NewStep("mode").
				Text("University Schedule", wasmplugin.StyleHeader).
				Options("Choose view mode:",
					wasmplugin.Opt("Quick (today)", "quick"),
					wasmplugin.Opt("By date", "by_date"),
				),

			// 2. Building (paginated).
			wasmplugin.NewStep("building").
				Text("Select building:", wasmplugin.StyleSubheader).
				PaginatedOptions("Building:", 2, buildingPages),

			// 3. Room (custom validation).
			wasmplugin.NewStep("room").
				Text("Enter room number:", wasmplugin.StylePlain).
				ValidateFunc(func(ctx *wasmplugin.CallbackContext) bool {
					return isDigits(ctx.Input) && len(ctx.Input) <= 4
				}),

			// 4. Date — only when mode="by_date".
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

func handleScheduleCmd(ctx *wasmplugin.CommandContext) error {
	mode := ctx.Params["mode"]
	building := ctx.Params["building"]
	room := ctx.Params["room"]

	// mode="by_date" → branch добавляет step "date", пользователь вводит дату.
	// mode="quick"   → branch не раскрывается, date отсутствует в params.
	date := ctx.Params["date"]
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
	text += generateScheduleForBuilding(building, room, date, ctx.Locale)

	ctx.Reply(text)
	return nil
}

// ---------------------------------------------------------------------------
// /find command — demonstrates branching, dynamic options, conditions,
// pagination, and nested branches.
//
// Flow:
//
//   Step: what  ("teacher" | "subject" | "room")
//   BranchOn("what"):
//     "teacher":
//       Step: building       (paginated)
//       Step: teacher        (DynamicOptions — depends on building)
//     "subject":
//       Step: subject        (paginated — all subjects)
//     "room":
//       Step: building       (paginated)
//       Step: floor          (ValidateFunc — digits only, 1-9)
//       ConditionalBranch:
//         building == "3" →  Step: wing ("east" | "west")
//   Step: notify  ("yes"|"no") — VisibleWhen(what != "room")
//
// ---------------------------------------------------------------------------

func findCommand() wasmplugin.Command {
	return wasmplugin.Command{
		Name:        "find",
		Description: "Find schedule by teacher, subject, or room",
		Nodes: []wasmplugin.Node{

			// ── 1. Search type ──────────────────────────────────
			wasmplugin.NewStep("what").
				Text("Search", wasmplugin.StyleHeader).
				Text("What do you want to find?", wasmplugin.StylePlain).
				Options("Search by:",
					wasmplugin.Opt("By teacher", "teacher"),
					wasmplugin.Opt("By subject", "subject"),
					wasmplugin.Opt("By room", "room"),
				),

			// ── 2. Branch on search type ────────────────────────
			wasmplugin.BranchOn("what",

				// ── 2a. Teacher path ────────────────────────────
				wasmplugin.Case("teacher",
					// Building (paginated, reuses shared provider).
					wasmplugin.NewStep("building").
						Text("Which building?", wasmplugin.StyleSubheader).
						PaginatedOptions("Building:", 2, buildingPages),

					// Teacher (dynamic — list depends on selected building).
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

				// ── 2b. Subject path ────────────────────────────
				wasmplugin.Case("subject",
					// Subject (paginated — 4 per page from the full list).
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

				// ── 2c. Room path ───────────────────────────────
				wasmplugin.Case("room",
					// Building.
					wasmplugin.NewStep("building").
						Text("Which building?", wasmplugin.StyleSubheader).
						PaginatedOptions("Building:", 2, buildingPages),

					// Floor (free-text with custom validation).
					wasmplugin.NewStep("floor").
						Text("Enter floor number (1-9):", wasmplugin.StylePlain).
						ValidateFunc(func(ctx *wasmplugin.CallbackContext) bool {
							return len(ctx.Input) == 1 && ctx.Input[0] >= '1' && ctx.Input[0] <= '9'
						}),

					// Wing — only building 3 has wings.
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
						// Otherwise: no extra steps for buildings 1 & 2.
					),
				),
			),

			// ── 3. Notification opt-in (not shown for "room" search) ─
			wasmplugin.NewStep("notify").
				Text("Enable notifications for this search?", wasmplugin.StylePlain).
				Options("Notify:",
					wasmplugin.Opt("Yes", "yes"),
					wasmplugin.Opt("No", "no"),
				).
				VisibleWhen(wasmplugin.ParamNeq("what", "room")),
		},

		Handler: func(ctx *wasmplugin.CommandContext) error {
			ctx.Log("find: " + ctx.Params["what"])
			ctx.Reply(handleFind(ctx.Locale, ctx.Params))
			return nil
		},
	}
}
