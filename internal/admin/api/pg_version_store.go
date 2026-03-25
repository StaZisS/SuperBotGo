package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PgVersionStore struct {
	pool *pgxpool.Pool
}

func NewPgVersionStore(pool *pgxpool.Pool) *PgVersionStore {
	return &PgVersionStore{pool: pool}
}

func (s *PgVersionStore) SaveVersion(ctx context.Context, rec VersionRecord) (int64, error) {
	configJSON := json.RawMessage("null")
	if len(rec.ConfigJSON) > 0 {
		configJSON = rec.ConfigJSON
	}

	permissions := rec.Permissions
	if permissions == nil {
		permissions = []string{}
	}

	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO wasm_plugin_versions (plugin_id, version, wasm_key, wasm_hash, config_json, permissions, changelog)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`,
		rec.PluginID,
		rec.Version,
		rec.WasmKey,
		rec.WasmHash,
		configJSON,
		permissions,
		rec.Changelog,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert plugin version: %w", err)
	}
	return id, nil
}

func (s *PgVersionStore) ListVersions(ctx context.Context, pluginID string) ([]VersionRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, plugin_id, version, wasm_key, wasm_hash, config_json, permissions, changelog, created_at
		FROM wasm_plugin_versions
		WHERE plugin_id = $1
		ORDER BY created_at DESC
	`, pluginID)
	if err != nil {
		return nil, fmt.Errorf("list versions for %q: %w", pluginID, err)
	}
	defer rows.Close()

	var records []VersionRecord
	for rows.Next() {
		var rec VersionRecord
		if err := rows.Scan(
			&rec.ID,
			&rec.PluginID,
			&rec.Version,
			&rec.WasmKey,
			&rec.WasmHash,
			&rec.ConfigJSON,
			&rec.Permissions,
			&rec.Changelog,
			&rec.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan version row: %w", err)
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate version rows: %w", err)
	}
	return records, nil
}

func (s *PgVersionStore) GetVersion(ctx context.Context, id int64) (VersionRecord, error) {
	var rec VersionRecord
	err := s.pool.QueryRow(ctx, `
		SELECT id, plugin_id, version, wasm_key, wasm_hash, config_json, permissions, changelog, created_at
		FROM wasm_plugin_versions
		WHERE id = $1
	`, id).Scan(
		&rec.ID,
		&rec.PluginID,
		&rec.Version,
		&rec.WasmKey,
		&rec.WasmHash,
		&rec.ConfigJSON,
		&rec.Permissions,
		&rec.Changelog,
		&rec.CreatedAt,
	)
	if err != nil {
		return VersionRecord{}, fmt.Errorf("get version %d: %w", id, err)
	}
	return rec, nil
}

func (s *PgVersionStore) DeleteVersion(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM wasm_plugin_versions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete version %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("version %d not found", id)
	}
	return nil
}

func (s *PgVersionStore) DeleteVersionsByPlugin(ctx context.Context, pluginID string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM wasm_plugin_versions WHERE plugin_id = $1`, pluginID); err != nil {
		return fmt.Errorf("delete versions for plugin %q: %w", pluginID, err)
	}
	return nil
}

var _ VersionStore = (*PgVersionStore)(nil)
