-- +goose Up

-- ============================================================
-- Настройки команд плагинов (вкл/выкл + policy expressions)
-- Контроль доступа реализован через policy expressions (evaluator.go)
-- ============================================================

CREATE TABLE plugin_command_settings (
    id            BIGSERIAL    PRIMARY KEY,
    plugin_id     VARCHAR(255) NOT NULL,
    command_name  VARCHAR(255) NOT NULL,
    enabled       BOOLEAN      NOT NULL DEFAULT true,
    policy_expression TEXT,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (plugin_id, command_name)
);

CREATE INDEX idx_plugin_cmd_settings_plugin ON plugin_command_settings(plugin_id);

COMMENT ON TABLE plugin_command_settings IS 'Вкл/выкл команд плагинов и policy expressions';

-- +goose Down
DROP TABLE IF EXISTS plugin_command_settings;
