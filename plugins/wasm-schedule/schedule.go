package main

import (
	"embed"
	"fmt"

	"github.com/superbot/wasmplugin"
)

type scheduleEntry struct {
	Time    string
	Subject string
	Teacher string
}

// seedSchedule is used to populate the database on first run.
var seedSchedule = map[string][]scheduleEntry{
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

//go:embed i18n/*.toml
var i18nFS embed.FS

var cat = wasmplugin.NewCatalog("en").
	LoadFS(i18nFS, "i18n")

func generateScheduleForBuilding(entries []scheduleEntry, building, room, date, locale string) string {
	tr := cat.Tr(locale)
	header := fmt.Sprintf("%s\n%s %s, %s %s, %s\n",
		tr("schedule"),
		tr("building"), building,
		tr("room"), room,
		date)

	if len(entries) == 0 {
		return header + "\n" + tr("no_classes")
	}

	result := header + "\n"
	for _, e := range entries {
		result += fmt.Sprintf("%s  %s (%s)\n", e.Time, tr(e.Subject), e.Teacher)
	}
	return result
}
