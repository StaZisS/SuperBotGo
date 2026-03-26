-- +goose Up
ALTER TABLE wasm_plugins DROP COLUMN IF EXISTS permissions;
ALTER TABLE wasm_plugins DROP COLUMN IF EXISTS schema_version;
ALTER TABLE wasm_plugin_versions DROP COLUMN IF EXISTS permissions;

-- +goose Down
ALTER TABLE wasm_plugins ADD COLUMN permissions text[] NOT NULL DEFAULT '{}';
ALTER TABLE wasm_plugins ADD COLUMN schema_version integer NOT NULL DEFAULT 0;
ALTER TABLE wasm_plugin_versions ADD COLUMN permissions text[] NOT NULL DEFAULT '{}';
