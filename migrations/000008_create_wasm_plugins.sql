-- +goose Up
CREATE TABLE wasm_plugins (
    id             VARCHAR(255) PRIMARY KEY,
    wasm_key       TEXT         NOT NULL,
    config_json    JSONB,
    permissions    TEXT[]       NOT NULL DEFAULT '{}',
    enabled        BOOLEAN      NOT NULL DEFAULT true,
    schema_version INT          NOT NULL DEFAULT 0,
    wasm_hash      VARCHAR(64)  NOT NULL,
    installed_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS wasm_plugins;
