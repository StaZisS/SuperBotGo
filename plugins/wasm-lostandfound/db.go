package main

import "database/sql"

func openDB() (*sql.DB, error) {
	return sql.Open("superbot", "")
}

type lostItem struct {
	ID          int
	Title       string
	Description string
	PhotoID     string
	Location    string
	Status      string
	CreatedBy   int64
}

func dbInsertItem(db *sql.DB, item lostItem) (int, error) {
	var id int
	err := db.QueryRow(
		`INSERT INTO lost_items (title, description, photo_id, location, created_by)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		item.Title, item.Description, item.PhotoID, item.Location, item.CreatedBy,
	).Scan(&id)
	return id, err
}

func dbActiveItems(db *sql.DB) ([]lostItem, error) {
	rows, err := db.Query(
		`SELECT id, title, description, photo_id, location, status, created_by
		 FROM lost_items WHERE status = 'active' ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanItems(rows)
}

func dbItemByID(db *sql.DB, id int) (*lostItem, error) {
	row := db.QueryRow(
		`SELECT id, title, description, photo_id, location, status, created_by
		 FROM lost_items WHERE id = $1`, id)
	var item lostItem
	err := row.Scan(&item.ID, &item.Title, &item.Description, &item.PhotoID,
		&item.Location, &item.Status, &item.CreatedBy)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func dbItemsByUser(db *sql.DB, userID int64) ([]lostItem, error) {
	rows, err := db.Query(
		`SELECT id, title, description, photo_id, location, status, created_by
		 FROM lost_items WHERE created_by = $1 ORDER BY id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanItems(rows)
}

func dbResolveItem(db *sql.DB, id int) error {
	_, err := db.Exec(`UPDATE lost_items SET status = 'resolved' WHERE id = $1`, id)
	return err
}

func scanItems(rows *sql.Rows) ([]lostItem, error) {
	var items []lostItem
	for rows.Next() {
		var item lostItem
		if err := rows.Scan(&item.ID, &item.Title, &item.Description,
			&item.PhotoID, &item.Location, &item.Status, &item.CreatedBy); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
