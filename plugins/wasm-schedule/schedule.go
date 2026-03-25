package main

import "fmt"

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

var t = map[string]map[string]string{
	"en": {
		"schedule":   "Schedule",
		"building":   "Building",
		"room":       "Room",
		"no_classes": "No classes scheduled",

		"Linear Algebra":   "Linear Algebra",
		"Programming":      "Programming",
		"Physics":          "Physics",
		"English":          "English",
		"Databases":        "Databases",
		"OS":               "OS",
		"Networks":         "Networks",
		"Machine Learning": "Machine Learning",
		"Statistics":       "Statistics",
		"Algorithms":       "Algorithms",
		"Seminar":          "Seminar",

		"quick_today": "Quick (today)",
		"by_date":     "By date",
		"by_teacher":  "By teacher",
		"by_subject":  "By subject",
		"by_room":     "By room",
		"east_wing":   "East wing",
		"west_wing":   "West wing",
		"yes":         "Yes",
		"no":          "No",
	},
	"ru": {
		"schedule":   "Расписание",
		"building":   "Корпус",
		"room":       "Аудитория",
		"no_classes": "Занятий нет",

		"Linear Algebra":   "Линейная алгебра",
		"Programming":      "Программирование",
		"Physics":          "Физика",
		"English":          "Английский язык",
		"Databases":        "Базы данных",
		"OS":               "Операционные системы",
		"Networks":         "Компьютерные сети",
		"Machine Learning": "Машинное обучение",
		"Statistics":       "Статистика",
		"Algorithms":       "Алгоритмы",
		"Seminar":          "Семинар",

		"quick_today": "Быстрый (сегодня)",
		"by_date":     "По дате",
		"by_teacher":  "По преподавателю",
		"by_subject":  "По предмету",
		"by_room":     "По аудитории",
		"east_wing":   "Восточное крыло",
		"west_wing":   "Западное крыло",
		"yes":         "Да",
		"no":          "Нет",
	},
}

func tr(locale, key string) string {
	if lang, ok := t[locale]; ok {
		if val, ok := lang[key]; ok {
			return val
		}
	}
	if val, ok := t["en"][key]; ok {
		return val
	}
	return key
}

func generateScheduleForBuilding(entries []scheduleEntry, building, room, date, locale string) string {
	header := fmt.Sprintf("%s\n%s %s, %s %s, %s\n",
		tr(locale, "schedule"),
		tr(locale, "building"), building,
		tr(locale, "room"), room,
		date)

	if len(entries) == 0 {
		return header + "\n" + tr(locale, "no_classes")
	}

	result := header + "\n"
	for _, e := range entries {
		result += fmt.Sprintf("%s  %s (%s)\n", e.Time, tr(locale, e.Subject), e.Teacher)
	}
	return result
}
