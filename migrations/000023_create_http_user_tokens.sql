-- +goose Up

CREATE TABLE http_user_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES global_users(id) ON DELETE CASCADE,
    public_id VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    secret_hash VARCHAR(128) NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_http_user_tokens_user_id ON http_user_tokens (user_id);
CREATE INDEX idx_http_user_tokens_active ON http_user_tokens (active);

COMMENT ON TABLE http_user_tokens IS 'User bearer tokens for HTTP trigger authentication';

-- +goose Down

DROP TABLE IF EXISTS http_user_tokens;
