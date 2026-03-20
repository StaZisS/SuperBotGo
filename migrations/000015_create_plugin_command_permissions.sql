-- +goose Up

-- ============================================================
-- Настройки команд плагинов (вкл/выкл)
-- Контроль доступа реализован через authorization_tuples (ReBAC):
--   object_type = 'plugin_command'
--   object_id   = '{plugin_id}:{command_name}'
--   relation    = 'executor'
--   subject_type / subject_id = кому разрешено (user, group, stream, faculty, ...)
-- ============================================================

CREATE TABLE plugin_command_settings (
    id            BIGSERIAL    PRIMARY KEY,
    plugin_id     VARCHAR(255) NOT NULL,
    command_name  VARCHAR(255) NOT NULL,
    enabled       BOOLEAN      NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (plugin_id, command_name)
);

CREATE INDEX idx_plugin_cmd_settings_plugin ON plugin_command_settings(plugin_id);

COMMENT ON TABLE plugin_command_settings IS 'Вкл/выкл команд плагинов. Доступ к командам — через authorization_tuples с object_type=plugin_command';

-- +goose Down
DROP TABLE IF EXISTS plugin_command_settings;
