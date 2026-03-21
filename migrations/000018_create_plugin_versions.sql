-- +goose Up
CREATE TABLE wasm_plugin_versions (
    id           BIGSERIAL    PRIMARY KEY,
    plugin_id    VARCHAR(255) NOT NULL,
    version      VARCHAR(255) NOT NULL DEFAULT '',
    wasm_key     TEXT         NOT NULL,
    wasm_hash    VARCHAR(64)  NOT NULL,
    config_json  JSONB,
    permissions  TEXT[]       NOT NULL DEFAULT '{}',
    changelog    TEXT         NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT fk_plugin_versions_plugin
        FOREIGN KEY (plugin_id) REFERENCES wasm_plugins(id) ON DELETE CASCADE
);

CREATE INDEX idx_plugin_versions_plugin_id ON wasm_plugin_versions(plugin_id);

-- +goose Down
DROP TABLE IF EXISTS wasm_plugin_versions;
