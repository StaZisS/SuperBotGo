-- +goose Up
CREATE TABLE global_users (
    id BIGSERIAL PRIMARY KEY,
    tsu_accounts_id VARCHAR(255),
    primary_channel VARCHAR(50) NOT NULL DEFAULT 'TELEGRAM',
    profile_data TEXT,
    locale VARCHAR(10),
    role VARCHAR(50) NOT NULL DEFAULT 'USER'
);

-- +goose Down
DROP TABLE IF EXISTS global_users;
