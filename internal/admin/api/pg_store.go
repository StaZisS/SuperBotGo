package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PgPluginStore struct {
	pool *pgxpool.Pool
}

func NewPgPluginStore(pool *pgxpool.Pool) *PgPluginStore {
	return &PgPluginStore{pool: pool}
}

func (s *PgPluginStore) SavePlugin(ctx context.Context, record PluginRecord) error {
	configJSON := json.RawMessage("null")
	if len(record.ConfigJSON) > 0 {
		configJSON = record.ConfigJSON
	}

	permissions := record.Permissions
	if permissions == nil {
		permissions = []string{}
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO wasm_plugins (id, wasm_key, config_json, permissions, enabled, schema_version, wasm_hash, installed_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			wasm_key       = EXCLUDED.wasm_key,
			config_json    = EXCLUDED.config_json,
			permissions    = EXCLUDED.permissions,
			enabled        = EXCLUDED.enabled,
			schema_version = EXCLUDED.schema_version,
			wasm_hash      = EXCLUDED.wasm_hash,
			updated_at     = EXCLUDED.updated_at
	`,
		record.ID,
		record.WasmKey,
		configJSON,
		permissions,
		record.Enabled,
		record.SchemaVersion,
		record.WasmHash,
		record.InstalledAt,
		record.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert plugin %q: %w", record.ID, err)
	}
	return nil
}

func (s *PgPluginStore) GetPlugin(ctx context.Context, id string) (PluginRecord, error) {
	var rec PluginRecord
	err := s.pool.QueryRow(ctx, `
		SELECT id, wasm_key, config_json, permissions, enabled, schema_version, wasm_hash, installed_at, updated_at
		FROM wasm_plugins
		WHERE id = $1
	`, id).Scan(
		&rec.ID,
		&rec.WasmKey,
		&rec.ConfigJSON,
		&rec.Permissions,
		&rec.Enabled,
		&rec.SchemaVersion,
		&rec.WasmHash,
		&rec.InstalledAt,
		&rec.UpdatedAt,
	)
	if err != nil {
		return PluginRecord{}, fmt.Errorf("get plugin %q: %w", id, err)
	}
	return rec, nil
}

func (s *PgPluginStore) ListPlugins(ctx context.Context) ([]PluginRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, wasm_key, config_json, permissions, enabled, schema_version, wasm_hash, installed_at, updated_at
		FROM wasm_plugins
		ORDER BY installed_at
	`)
	if err != nil {
		return nil, fmt.Errorf("list plugins: %w", err)
	}
	defer rows.Close()

	var records []PluginRecord
	for rows.Next() {
		var rec PluginRecord
		if err := rows.Scan(
			&rec.ID,
			&rec.WasmKey,
			&rec.ConfigJSON,
			&rec.Permissions,
			&rec.Enabled,
			&rec.SchemaVersion,
			&rec.WasmHash,
			&rec.InstalledAt,
			&rec.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan plugin row: %w", err)
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plugin rows: %w", err)
	}
	return records, nil
}

func (s *PgPluginStore) DeletePlugin(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM wasm_plugins WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete plugin %q: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("plugin %q not found", id)
	}
	return nil
}

var _ PluginStore = (*PgPluginStore)(nil)
