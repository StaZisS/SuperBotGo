-- +goose Up
CREATE TABLE wasm_plugins (
    id             VARCHAR(255) PRIMARY KEY,
    wasm_key       TEXT         NOT NULL,
    config_json    JSONB,
    enabled        BOOLEAN      NOT NULL DEFAULT true,
    wasm_hash      VARCHAR(64)  NOT NULL,
    installed_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS wasm_plugins;
