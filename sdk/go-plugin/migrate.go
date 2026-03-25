package wasmplugin

import (
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

// MigrationsFromFS reads goose-formatted SQL migration files from an fs.FS
// and returns them as []SQLMigration. This is typically used with embed.FS:
//
//	//go:embed migrations/*.sql
//	var migrationsFS embed.FS
//
//	Plugin{
//	    Migrations: wasmplugin.MigrationsFromFS(migrationsFS, "migrations"),
//	}
//
// Files must follow the goose naming convention: {version}_{description}.sql
// (e.g. "000001_create_users.sql"). Content must use -- +goose Up / -- +goose Down
// markers.
func MigrationsFromFS(fsys fs.FS, dir string) []SQLMigration {
	if dir != "" && dir != "." {
		sub, err := fs.Sub(fsys, dir)
		if err != nil {
			return nil
		}
		fsys = sub
	}

	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil
	}

	var migrations []SQLMigration
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}

		version, description := parseGooseFilename(name)
		if version < 0 {
			continue
		}

		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			continue
		}

		up, down := parseGooseSections(string(data))
		if up == "" {
			continue
		}

		migrations = append(migrations, SQLMigration{
			Version:     version,
			Description: description,
			Up:          up,
			Down:        down,
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations
}

// parseGooseFilename extracts version and description from a goose filename.
// E.g. "000001_create_users.sql" → (1, "create_users")
// Returns (-1, "") if the filename doesn't match the expected pattern.
func parseGooseFilename(name string) (int, string) {
	name = strings.TrimSuffix(name, path.Ext(name)) // strip .sql
	idx := strings.Index(name, "_")
	if idx <= 0 {
		return -1, ""
	}
	v, err := strconv.Atoi(name[:idx])
	if err != nil {
		return -1, ""
	}
	return v, name[idx+1:]
}

// parseGooseSections splits goose SQL file content into Up and Down sections.
// Content between "-- +goose Up" and "-- +goose Down" (or EOF) is the Up SQL.
// Content after "-- +goose Down" until EOF is the Down SQL.
// Goose directives like "-- +goose StatementBegin/End" are preserved as-is.
func parseGooseSections(content string) (up, down string) {
	lines := strings.Split(content, "\n")
	var section string // "", "up", "down"
	var upLines, downLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case "-- +goose Up":
			section = "up"
			continue
		case "-- +goose Down":
			section = "down"
			continue
		}
		switch section {
		case "up":
			upLines = append(upLines, line)
		case "down":
			downLines = append(downLines, line)
		}
	}

	up = strings.TrimSpace(strings.Join(upLines, "\n"))
	down = strings.TrimSpace(strings.Join(downLines, "\n"))
	return
}
