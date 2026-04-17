-- +goose Up
CREATE TABLE wasm_plugin_metadata (
    plugin_id     VARCHAR(255) PRIMARY KEY,
    name          TEXT         NOT NULL DEFAULT '',
    version       VARCHAR(255) NOT NULL DEFAULT '',
    sdk_version   INTEGER      NOT NULL DEFAULT 0,
    meta_json     JSONB,
    config_schema JSONB,
    requirements  JSONB,
    triggers      JSONB,
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT fk_wasm_plugin_metadata_plugin
        FOREIGN KEY (plugin_id) REFERENCES wasm_plugins(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE IF EXISTS wasm_plugin_metadata;
