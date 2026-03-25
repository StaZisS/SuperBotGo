package wasmplugin

import (
	"testing"
	"testing/fstest"
)

func TestParseGooseFilename(t *testing.T) {
	tests := []struct {
		name     string
		wantVer  int
		wantDesc string
	}{
		{"000001_create_users.sql", 1, "create_users"},
		{"000020_add_email_column.sql", 20, "add_email_column"},
		{"1_init.sql", 1, "init"},
		{"bad.sql", -1, ""},
		{"_no_version.sql", -1, ""},
	}
	for _, tt := range tests {
		v, d := parseGooseFilename(tt.name)
		if v != tt.wantVer || d != tt.wantDesc {
			t.Errorf("parseGooseFilename(%q) = (%d, %q), want (%d, %q)",
				tt.name, v, d, tt.wantVer, tt.wantDesc)
		}
	}
}

func TestParseGooseSections(t *testing.T) {
	content := `-- +goose Up
CREATE TABLE users (id INT);

-- +goose Down
DROP TABLE users;
`
	up, down := parseGooseSections(content)
	if up != "CREATE TABLE users (id INT);" {
		t.Errorf("up = %q", up)
	}
	if down != "DROP TABLE users;" {
		t.Errorf("down = %q", down)
	}
}

func TestParseGooseSections_NoDown(t *testing.T) {
	content := `-- +goose Up
ALTER TABLE users ADD COLUMN email TEXT;
`
	up, down := parseGooseSections(content)
	if up != "ALTER TABLE users ADD COLUMN email TEXT;" {
		t.Errorf("up = %q", up)
	}
	if down != "" {
		t.Errorf("down = %q, want empty", down)
	}
}

func TestParseGooseSections_StatementBeginEnd(t *testing.T) {
	content := `-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION hello() RETURNS void AS $$
BEGIN
  RAISE NOTICE 'hello';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS hello();
`
	up, down := parseGooseSections(content)
	if up == "" {
		t.Error("up is empty")
	}
	if !contains(up, "-- +goose StatementBegin") {
		t.Error("up should preserve StatementBegin marker")
	}
	if down != "DROP FUNCTION IF EXISTS hello();" {
		t.Errorf("down = %q", down)
	}
}

func TestMigrationsFromFS(t *testing.T) {
	fsys := fstest.MapFS{
		"migrations/000001_create_users.sql": &fstest.MapFile{
			Data: []byte("-- +goose Up\nCREATE TABLE users (id INT);\n\n-- +goose Down\nDROP TABLE users;\n"),
		},
		"migrations/000002_add_email.sql": &fstest.MapFile{
			Data: []byte("-- +goose Up\nALTER TABLE users ADD COLUMN email TEXT;\n"),
		},
		"migrations/readme.txt": &fstest.MapFile{
			Data: []byte("not a migration"),
		},
	}

	ms := MigrationsFromFS(fsys, "migrations")
	if len(ms) != 2 {
		t.Fatalf("got %d migrations, want 2", len(ms))
	}

	if ms[0].Version != 1 || ms[0].Description != "create_users" {
		t.Errorf("ms[0] = %+v", ms[0])
	}
	if ms[0].Down != "DROP TABLE users;" {
		t.Errorf("ms[0].Down = %q", ms[0].Down)
	}

	if ms[1].Version != 2 || ms[1].Description != "add_email" {
		t.Errorf("ms[1] = %+v", ms[1])
	}
	if ms[1].Down != "" {
		t.Errorf("ms[1].Down = %q, want empty", ms[1].Down)
	}
}

func TestMigrationsFromFS_Sorted(t *testing.T) {
	fsys := fstest.MapFS{
		"000003_third.sql": &fstest.MapFile{
			Data: []byte("-- +goose Up\nSELECT 3;\n"),
		},
		"000001_first.sql": &fstest.MapFile{
			Data: []byte("-- +goose Up\nSELECT 1;\n"),
		},
		"000002_second.sql": &fstest.MapFile{
			Data: []byte("-- +goose Up\nSELECT 2;\n"),
		},
	}

	ms := MigrationsFromFS(fsys, ".")
	if len(ms) != 3 {
		t.Fatalf("got %d migrations, want 3", len(ms))
	}
	if ms[0].Version != 1 || ms[1].Version != 2 || ms[2].Version != 3 {
		t.Errorf("not sorted: %d, %d, %d", ms[0].Version, ms[1].Version, ms[2].Version)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
