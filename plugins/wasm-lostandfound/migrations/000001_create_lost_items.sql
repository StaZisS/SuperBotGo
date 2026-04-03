-- +goose Up
CREATE TABLE IF NOT EXISTS lost_items (
    id          SERIAL PRIMARY KEY,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    photo_id    TEXT NOT NULL DEFAULT '',
    location    TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'active',
    created_by  BIGINT NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_lost_items_status ON lost_items (status);

-- +goose Down
DROP TABLE IF EXISTS lost_items;
