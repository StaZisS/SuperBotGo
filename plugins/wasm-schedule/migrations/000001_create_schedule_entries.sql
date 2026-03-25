-- +goose Up
CREATE TABLE IF NOT EXISTS schedule_entries (
    id SERIAL PRIMARY KEY,
    building TEXT NOT NULL,
    time_slot TEXT NOT NULL,
    subject TEXT NOT NULL,
    teacher TEXT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS schedule_entries;
