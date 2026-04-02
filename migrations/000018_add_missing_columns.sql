-- +goose Up
ALTER TABLE global_users
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();

ALTER TABLE channel_accounts
    ADD COLUMN IF NOT EXISTS username VARCHAR(255),
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();

ALTER TABLE user_roles
    ADD COLUMN IF NOT EXISTS scope VARCHAR(255),
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- +goose Down
ALTER TABLE user_roles
    DROP COLUMN IF EXISTS created_at,
    DROP COLUMN IF EXISTS scope;

ALTER TABLE channel_accounts
    DROP COLUMN IF EXISTS created_at,
    DROP COLUMN IF EXISTS username;

ALTER TABLE global_users
    DROP COLUMN IF EXISTS created_at;
