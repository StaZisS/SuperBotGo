-- +goose Up
CREATE TABLE channel_accounts (
    id BIGSERIAL PRIMARY KEY,
    channel_type VARCHAR(50) NOT NULL,
    channel_user_id VARCHAR(255) NOT NULL,
    global_user_id BIGINT NOT NULL,
    CONSTRAINT fk_channel_accounts_global_user FOREIGN KEY (global_user_id) REFERENCES global_users(id)
);

ALTER TABLE channel_accounts
    ADD CONSTRAINT uq_channel_accounts_type_user_id UNIQUE (channel_type, channel_user_id);

CREATE INDEX idx_channel_accounts_global_user_id ON channel_accounts(global_user_id);

-- +goose Down
DROP TABLE IF EXISTS channel_accounts;
