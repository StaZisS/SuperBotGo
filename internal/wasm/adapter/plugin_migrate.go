package adapter

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"testing/fstest"

	wasmrt "SuperBotGo/internal/wasm/runtime"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"
)

var unsafeCharsRe = regexp.MustCompile(`[^a-z0-9_]`)

func sanitizeDescription(desc string) string {
	s := strings.ToLower(strings.TrimSpace(desc))
	s = strings.ReplaceAll(s, " ", "_")
	s = unsafeCharsRe.ReplaceAllString(s, "")
	if s == "" {
		s = "migration"
	}
	return s
}

// runPluginMigrations runs goose SQL migrations declared in plugin metadata.
// Each plugin gets its own goose version table (_goose_plugin_{pluginID}).
func runPluginMigrations(ctx context.Context, pluginID, dsn string, migrations []wasmrt.MigrationDef) error {
	if len(migrations) == 0 {
		return nil
	}

	// Build in-memory FS with goose-formatted SQL files.
	fsys := make(fstest.MapFS, len(migrations))
	for _, m := range migrations {
		filename := fmt.Sprintf("%06d_%s.sql", m.Version, sanitizeDescription(m.Description))

		content := "-- +goose Up\n" + m.Up + "\n"
		if m.Down != "" {
			content += "\n-- +goose Down\n" + m.Down + "\n"
		}

		fsys[filename] = &fstest.MapFile{Data: []byte(content)}
	}

	// Open a dedicated connection for goose (separate from the plugin's pgxpool).
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open db for migrations: %w", err)
	}
	defer db.Close()

	// Per-plugin goose tracking table.
	tableName := "_goose_plugin_" + pluginID
	store, err := database.NewStore(database.DialectPostgres, tableName)
	if err != nil {
		return fmt.Errorf("create goose store: %w", err)
	}

	provider, err := goose.NewProvider("", db, fsys, goose.WithStore(store))
	if err != nil {
		return fmt.Errorf("create goose provider: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	for _, r := range results {
		slog.Info("wasm: plugin migration applied",
			"plugin", pluginID,
			"version", r.Source.Version,
			"duration_ms", r.Duration.Milliseconds(),
		)
	}

	if len(results) == 0 {
		slog.Debug("wasm: no pending migrations", "plugin", pluginID)
	}

	return nil
}
