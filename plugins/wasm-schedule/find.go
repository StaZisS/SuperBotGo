package main

import (
	"fmt"
	"strings"

	"github.com/superbot/wasmplugin"
)

// seedTeachers is used only as fallback data for dynamic options.
var seedTeachers = map[string][]string{
	"1": {"Ivanov A.B.", "Petrov S.D.", "Sidorova E.V.", "Smith J.K."},
	"2": {"Kozlov I.P.", "Morozova T.N.", "Volkov D.A."},
	"3": {"Kuznetsov R.V.", "Orlova M.S.", "Novikov P.E.", "Fedorov A.A."},
}

// seedSubjects is used only as fallback data for dynamic options.
var seedSubjects = []string{
	"Linear Algebra", "Programming", "Physics", "English",
	"Databases", "OS", "Networks",
	"Machine Learning", "Statistics", "Algorithms", "Seminar",
}

func fallbackTeacherOptions(building string) []wasmplugin.Option {
	names := seedTeachers[building]
	opts := make([]wasmplugin.Option, len(names))
	for i, name := range names {
		opts[i] = wasmplugin.Option{Label: name, Value: name}
	}
	return opts
}

func fallbackSubjectPage(ctx *wasmplugin.CallbackContext) wasmplugin.OptionsPage {
	pageSize := 4
	start := ctx.Page * pageSize
	if start >= len(seedSubjects) {
		return wasmplugin.OptionsPage{}
	}
	end := start + pageSize
	if end > len(seedSubjects) {
		end = len(seedSubjects)
	}
	opts := make([]wasmplugin.Option, end-start)
	for i, s := range seedSubjects[start:end] {
		opts[i] = wasmplugin.Option{Label: tr(ctx.Locale, s), Value: s}
	}
	return wasmplugin.OptionsPage{
		Options: opts,
		HasMore: end < len(seedSubjects),
	}
}

func handleFind(locale string, params map[string]string) string {
	db, err := openDB()
	if err != nil {
		return "Database error"
	}
	defer db.Close()

	what := params["what"]
	var b strings.Builder

	switch what {
	case "teacher":
		building := params["building"]
		teacher := params["teacher"]
		b.WriteString(fmt.Sprintf("%s: %s\n", tr(locale, "building"), building))
		b.WriteString(fmt.Sprintf("Teacher: %s\n\n", teacher))
		entries, err := dbScheduleByBuilding(db, building)
		if err == nil {
			for _, e := range entries {
				if e.Teacher == teacher {
					b.WriteString(fmt.Sprintf("  %s  %s\n", e.Time, tr(locale, e.Subject)))
				}
			}
		}
		if b.Len() == 0 {
			b.WriteString(tr(locale, "no_classes"))
		}

	case "subject":
		subject := params["subject"]
		b.WriteString(fmt.Sprintf("Subject: %s\n\n", tr(locale, subject)))
		buildings, _ := dbAllBuildings(db)
		for _, bld := range buildings {
			entries, err := dbScheduleByBuilding(db, bld)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if e.Subject == subject {
					b.WriteString(fmt.Sprintf("  %s %s — %s (%s)\n",
						tr(locale, "building"), bld, e.Time, e.Teacher))
				}
			}
		}

	case "room":
		building := params["building"]
		floor := params["floor"]
		wing := params["wing"]
		b.WriteString(fmt.Sprintf("%s %s, floor %s", tr(locale, "building"), building, floor))
		if wing != "" {
			b.WriteString(fmt.Sprintf(", wing %s", wing))
		}
		b.WriteString("\n\n")
		entries, _ := dbScheduleByBuilding(db, building)
		for _, e := range entries {
			b.WriteString(fmt.Sprintf("  %s  %s (%s)\n", e.Time, tr(locale, e.Subject), e.Teacher))
		}
	}

	if params["notify"] == "yes" {
		b.WriteString("\nNotifications enabled.")
	}

	return b.String()
}
