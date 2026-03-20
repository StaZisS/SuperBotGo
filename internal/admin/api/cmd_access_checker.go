package api

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"SuperBotGo/internal/model"
	"SuperBotGo/internal/policy"
)

// CommandAccessChecker проверяет доступ пользователя к командам плагинов.
//
// Логика:
//  1. enabled = false → запрет
//  2. policy_expression задано → вычисляем выражение (expr-lang)
//  3. Нет выражения → разрешено всем
type CommandAccessChecker struct {
	pool      *pgxpool.Pool
	evaluator *policy.Evaluator
}

func NewCommandAccessChecker(pool *pgxpool.Pool) *CommandAccessChecker {
	return &CommandAccessChecker{
		pool:      pool,
		evaluator: policy.NewEvaluator(pool),
	}
}

func (c *CommandAccessChecker) CanExecute(ctx context.Context, pluginID, commandName string, userID model.GlobalUserID) (bool, error) {
	// Читаем настройки команды
	var enabled bool
	var policyExpr *string
	err := c.pool.QueryRow(ctx, `
		SELECT enabled, policy_expression FROM plugin_command_settings
		WHERE plugin_id = $1 AND command_name = $2
	`, pluginID, commandName).Scan(&enabled, &policyExpr)

	if err == pgx.ErrNoRows {
		// Нет настроек — команда открыта всем
		return true, nil
	}
	if err != nil {
		return false, err
	}

	// Команда отключена
	if !enabled {
		return false, nil
	}

	// Есть policy expression — вычисляем
	if policyExpr != nil && *policyExpr != "" {
		ok, evalErr := c.evaluator.Evaluate(ctx, *policyExpr, userID)
		if evalErr != nil {
			slog.Warn("policy expression error",
				slog.String("plugin", pluginID),
				slog.String("command", commandName),
				slog.Any("error", evalErr))
			return false, nil // ошибка в выражении → запрет
		}
		return ok, nil
	}

	// enabled=true, выражения нет → открыто всем
	return true, nil
}
