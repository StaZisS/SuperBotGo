package main

import (
	"fmt"
	"strings"
)

// teachers per building, derived from the schedule data.
var teachers = map[string][]string{
	"1": {"Ivanov A.B.", "Petrov S.D.", "Sidorova E.V.", "Smith J.K."},
	"2": {"Kozlov I.P.", "Morozova T.N.", "Volkov D.A."},
	"3": {"Kuznetsov R.V.", "Orlova M.S.", "Novikov P.E.", "Fedorov A.A."},
}

// allSubjects is a flat list of every unique subject across all buildings.
var allSubjects = []string{
	"Linear Algebra", "Programming", "Physics", "English",
	"Databases", "OS", "Networks",
	"Machine Learning", "Statistics", "Algorithms", "Seminar",
}

func handleFind(locale string, params map[string]string) string {
	what := params["what"]
	var b strings.Builder

	switch what {
	case "teacher":
		building := params["building"]
		teacher := params["teacher"]
		b.WriteString(fmt.Sprintf("%s: %s\n", tr(locale, "building"), building))
		b.WriteString(fmt.Sprintf("Teacher: %s\n\n", teacher))
		for _, e := range schedule[building] {
			if e.Teacher == teacher {
				b.WriteString(fmt.Sprintf("  %s  %s\n", e.Time, tr(locale, e.Subject)))
			}
		}
		if b.Len() == 0 {
			b.WriteString(tr(locale, "no_classes"))
		}

	case "subject":
		subject := params["subject"]
		b.WriteString(fmt.Sprintf("Subject: %s\n\n", tr(locale, subject)))
		for bld, entries := range schedule {
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
		wing := params["wing"] // may be empty
		b.WriteString(fmt.Sprintf("%s %s, floor %s", tr(locale, "building"), building, floor))
		if wing != "" {
			b.WriteString(fmt.Sprintf(", wing %s", wing))
		}
		b.WriteString("\n\n")
		for _, e := range schedule[building] {
			b.WriteString(fmt.Sprintf("  %s  %s (%s)\n", e.Time, tr(locale, e.Subject), e.Teacher))
		}
	}

	if params["notify"] == "yes" {
		b.WriteString("\nNotifications enabled.")
	}

	return b.String()
}
