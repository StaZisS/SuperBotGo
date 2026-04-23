-- +goose Up

ALTER TABLE plugin_command_settings
    ADD COLUMN allow_user_keys BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN allow_service_keys BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE http_service_keys (
    id BIGSERIAL PRIMARY KEY,
    public_id VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    secret_hash VARCHAR(128) NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE http_service_key_scopes (
    service_key_id BIGINT NOT NULL REFERENCES http_service_keys(id) ON DELETE CASCADE,
    plugin_id VARCHAR(255) NOT NULL,
    trigger_name VARCHAR(255) NOT NULL,
    PRIMARY KEY (service_key_id, plugin_id, trigger_name)
);

CREATE INDEX idx_http_service_keys_active ON http_service_keys (active);
CREATE INDEX idx_http_service_key_scopes_plugin_trigger
    ON http_service_key_scopes (plugin_id, trigger_name);

COMMENT ON TABLE http_service_keys IS 'Service-to-service credentials for HTTP triggers';
COMMENT ON TABLE http_service_key_scopes IS 'Allowed plugin/trigger scopes for HTTP service keys';

-- +goose Down

DROP TABLE IF EXISTS http_service_key_scopes;
DROP TABLE IF EXISTS http_service_keys;

ALTER TABLE plugin_command_settings
    DROP COLUMN IF EXISTS allow_service_keys,
    DROP COLUMN IF EXISTS allow_user_keys;
