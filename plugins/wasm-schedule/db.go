package main

import "database/sql"

func openDB() (*sql.DB, error) {
	return sql.Open("superbot", "")
}

func ensureSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schedule_entries (
			id SERIAL PRIMARY KEY,
			building TEXT NOT NULL,
			time_slot TEXT NOT NULL,
			subject TEXT NOT NULL,
			teacher TEXT NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM schedule_entries").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	for building, entries := range seedSchedule {
		for _, e := range entries {
			if _, err := db.Exec(
				"INSERT INTO schedule_entries (building, time_slot, subject, teacher) VALUES ($1, $2, $3, $4)",
				building, e.Time, e.Subject, e.Teacher,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func dbScheduleByBuilding(db *sql.DB, building string) ([]scheduleEntry, error) {
	rows, err := db.Query(
		"SELECT time_slot, subject, teacher FROM schedule_entries WHERE building = $1 ORDER BY time_slot",
		building,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []scheduleEntry
	for rows.Next() {
		var e scheduleEntry
		if err := rows.Scan(&e.Time, &e.Subject, &e.Teacher); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func dbAllBuildings(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SELECT DISTINCT building FROM schedule_entries ORDER BY building")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buildings []string
	for rows.Next() {
		var b string
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		buildings = append(buildings, b)
	}
	return buildings, rows.Err()
}

func dbTeachersByBuilding(db *sql.DB, building string) ([]string, error) {
	rows, err := db.Query(
		"SELECT DISTINCT teacher FROM schedule_entries WHERE building = $1 ORDER BY teacher",
		building,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func dbAllSubjects(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SELECT DISTINCT subject FROM schedule_entries ORDER BY subject")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subjects []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		subjects = append(subjects, s)
	}
	return subjects, rows.Err()
}
