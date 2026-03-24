-- +goose Up
ALTER TABLE chat_references ADD COLUMN locale VARCHAR(10);

-- +goose Down
ALTER TABLE chat_references DROP COLUMN IF EXISTS locale;
