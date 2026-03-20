-- +goose Up

-- Поле для хранения произвольных правил доступа (expr-lang выражения).
-- Если задано — проверяется вместо / в дополнение к ReBAC-кортежам.
-- Примеры:
--   check("member", "faculty", "engineering")
--   user.nationality_type == "foreign" && check("dean", "department", "cs")
--   has_role("ADMIN") || is_member("group", "972203")
ALTER TABLE plugin_command_settings ADD COLUMN policy_expression TEXT;

COMMENT ON COLUMN plugin_command_settings.policy_expression IS 'Expr-lang выражение проверки доступа. Если задано — вычисляется при выполнении команды.';

-- +goose Down
ALTER TABLE plugin_command_settings DROP COLUMN IF EXISTS policy_expression;
