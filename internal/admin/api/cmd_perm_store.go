package api

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CommandSetting хранит настройку команды плагина.
type CommandSetting struct {
	ID               int64     `json:"id"`
	PluginID         string    `json:"plugin_id"`
	CommandName      string    `json:"command_name"`
	Enabled          bool      `json:"enabled"`
	PolicyExpression string    `json:"policy_expression"` // expr-lang выражение
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// CommandPermStore управляет настройками команд плагинов.
type CommandPermStore interface {
	ListCommandSettings(ctx context.Context, pluginID string) ([]CommandSetting, error)
	SetCommandEnabled(ctx context.Context, pluginID, commandName string, enabled bool) error
	SetPolicyExpression(ctx context.Context, pluginID, commandName, expression string) error
	GetPolicyExpression(ctx context.Context, pluginID, commandName string) (string, error)
}

// PgCommandPermStore реализует CommandPermStore на PostgreSQL.
type PgCommandPermStore struct {
	pool *pgxpool.Pool
}

func NewPgCommandPermStore(pool *pgxpool.Pool) *PgCommandPermStore {
	return &PgCommandPermStore{pool: pool}
}

// --- Настройки команд ---

func (s *PgCommandPermStore) ListCommandSettings(ctx context.Context, pluginID string) ([]CommandSetting, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, plugin_id, command_name, enabled, COALESCE(policy_expression, ''), created_at, updated_at
		FROM plugin_command_settings
		WHERE plugin_id = $1
		ORDER BY command_name
	`, pluginID)
	if err != nil {
		return nil, fmt.Errorf("list command settings for %q: %w", pluginID, err)
	}
	defer rows.Close()

	var settings []CommandSetting
	for rows.Next() {
		var s CommandSetting
		if err := rows.Scan(&s.ID, &s.PluginID, &s.CommandName, &s.Enabled, &s.PolicyExpression, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan command setting: %w", err)
		}
		settings = append(settings, s)
	}
	return settings, rows.Err()
}

func (s *PgCommandPermStore) SetCommandEnabled(ctx context.Context, pluginID, commandName string, enabled bool) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO plugin_command_settings (plugin_id, command_name, enabled, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (plugin_id, command_name) DO UPDATE SET
			enabled    = EXCLUDED.enabled,
			updated_at = now()
	`, pluginID, commandName, enabled)
	if err != nil {
		return fmt.Errorf("set command enabled %q/%q: %w", pluginID, commandName, err)
	}
	return nil
}

func (s *PgCommandPermStore) SetPolicyExpression(ctx context.Context, pluginID, commandName, expression string) error {
	var policyExpr *string
	if expression != "" {
		policyExpr = &expression
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO plugin_command_settings (plugin_id, command_name, enabled, policy_expression, updated_at)
		VALUES ($1, $2, true, $3, now())
		ON CONFLICT (plugin_id, command_name) DO UPDATE SET
			policy_expression = EXCLUDED.policy_expression,
			updated_at = now()
	`, pluginID, commandName, policyExpr)
	if err != nil {
		return fmt.Errorf("set policy expression %q/%q: %w", pluginID, commandName, err)
	}
	return nil
}

func (s *PgCommandPermStore) GetPolicyExpression(ctx context.Context, pluginID, commandName string) (string, error) {
	var expr *string
	err := s.pool.QueryRow(ctx, `
		SELECT policy_expression FROM plugin_command_settings
		WHERE plugin_id = $1 AND command_name = $2
	`, pluginID, commandName).Scan(&expr)
	if err != nil {
		return "", nil // no row = no expression
	}
	if expr == nil {
		return "", nil
	}
	return *expr, nil
}

var _ CommandPermStore = (*PgCommandPermStore)(nil)
