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
		opts[i] = cat.Opt(s, s)
	}
	return wasmplugin.OptionsPage{
		Options: opts,
		HasMore: end < len(seedSubjects),
	}
}

func handleFind(locale string, params map[string]string) string {
	db, err := openDB()
	if err != nil {
		return cat.T(locale, "error")
	}
	defer db.Close()

	tr := cat.Tr(locale)
	what := params["what"]
	var b strings.Builder

	switch what {
	case "teacher":
		building := params["building"]
		teacher := params["teacher"]
		b.WriteString(cat.T(locale, "find_building_line", "Building", tr("building"), "Bld", building))
		b.WriteString("\n")
		b.WriteString(cat.T(locale, "find_teacher_line", "Teacher", teacher))
		b.WriteString("\n\n")
		entries, err := dbScheduleByBuilding(db, building)
		if err == nil {
			for _, e := range entries {
				if e.Teacher == teacher {
					b.WriteString(fmt.Sprintf("  %s  %s\n", e.Time, tr(e.Subject)))
				}
			}
		}
		if b.Len() == 0 {
			b.WriteString(tr("no_classes"))
		}

	case "subject":
		subject := params["subject"]
		b.WriteString(cat.T(locale, "find_subject_line", "Subject", tr(subject)))
		b.WriteString("\n\n")
		buildings, _ := dbAllBuildings(db)
		for _, bld := range buildings {
			entries, err := dbScheduleByBuilding(db, bld)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if e.Subject == subject {
					b.WriteString(cat.T(locale, "find_entry", "Building", tr("building"), "Bld", bld, "Time", e.Time, "Teacher", e.Teacher))
					b.WriteString("\n")
				}
			}
		}

	case "room":
		building := params["building"]
		floor := params["floor"]
		wing := params["wing"]
		if wing != "" {
			b.WriteString(cat.T(locale, "find_room_wing", "Building", tr("building"), "Bld", building, "Floor", floor, "Wing", tr(wing)))
		} else {
			b.WriteString(cat.T(locale, "find_room_line", "Building", tr("building"), "Bld", building, "Floor", floor))
		}
		b.WriteString("\n\n")
		entries, _ := dbScheduleByBuilding(db, building)
		for _, e := range entries {
			b.WriteString(fmt.Sprintf("  %s  %s (%s)\n", e.Time, tr(e.Subject), e.Teacher))
		}
	}

	if params["notify"] == "yes" {
		b.WriteString("\n" + tr("notify_enabled"))
	}

	return b.String()
}
