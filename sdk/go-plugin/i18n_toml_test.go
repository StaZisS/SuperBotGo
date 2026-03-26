package wasmplugin

import (
	"testing"
	"testing/fstest"
)

func TestParseFlatTOML_Basic(t *testing.T) {
	input := []byte(`
# UI labels
"schedule" = "Schedule"
"building" = "Building"
greeting = "Hello, {{.Name}}!"
`)
	kv, err := parseFlatTOML(input)
	if err != nil {
		t.Fatalf("parseFlatTOML: %v", err)
	}
	tests := map[string]string{
		"schedule": "Schedule",
		"building": "Building",
		"greeting": "Hello, {{.Name}}!",
	}
	for k, want := range tests {
		if got := kv[k]; got != want {
			t.Errorf("key %q = %q, want %q", k, got, want)
		}
	}
}

func TestParseFlatTOML_DottedKey(t *testing.T) {
	input := []byte(`"schedule.header" = "Schedule for {{.V0}}"`)
	kv, err := parseFlatTOML(input)
	if err != nil {
		t.Fatalf("parseFlatTOML: %v", err)
	}
	if got := kv["schedule.header"]; got != "Schedule for {{.V0}}" {
		t.Errorf("got %q", got)
	}
}

func TestParseFlatTOML_Escapes(t *testing.T) {
	input := []byte(`msg = "line1\nline2\ttab\\"`)
	kv, err := parseFlatTOML(input)
	if err != nil {
		t.Fatalf("parseFlatTOML: %v", err)
	}
	want := "line1\nline2\ttab\\"
	if got := kv["msg"]; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseFlatTOML_EscapedQuoteInValue(t *testing.T) {
	input := []byte(`msg = "say \"hello\""`)
	kv, err := parseFlatTOML(input)
	if err != nil {
		t.Fatalf("parseFlatTOML: %v", err)
	}
	want := `say "hello"`
	if got := kv["msg"]; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseFlatTOML_EmptyValue(t *testing.T) {
	input := []byte(`empty = ""`)
	kv, err := parseFlatTOML(input)
	if err != nil {
		t.Fatalf("parseFlatTOML: %v", err)
	}
	if got := kv["empty"]; got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestParseFlatTOML_Errors(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"no equals", `key "value"`},
		{"no quote", `key = value`},
		{"unterminated key", `"key = "value"`},
		{"unterminated value", `key = "value`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseFlatTOML([]byte(tc.input))
			if err == nil {
				t.Errorf("expected error for input %q", tc.input)
			}
		})
	}
}

func TestCatalog_LoadFS(t *testing.T) {
	fsys := fstest.MapFS{
		"i18n/en.toml": {Data: []byte(`
schedule = "Schedule"
building = "Building"
`)},
		"i18n/ru.toml": {Data: []byte(`
schedule = "Расписание"
building = "Корпус"
`)},
	}

	cat := NewCatalog("en").LoadFS(fsys, "i18n")

	if got := cat.T("en", "schedule"); got != "Schedule" {
		t.Errorf("T(en, schedule) = %q", got)
	}
	if got := cat.T("ru", "building"); got != "Корпус" {
		t.Errorf("T(ru, building) = %q", got)
	}

	// L returns both locales.
	l := cat.L("schedule")
	if l["en"] != "Schedule" || l["ru"] != "Расписание" {
		t.Errorf("L(schedule) = %v", l)
	}
}

func TestCatalog_LoadFS_MergeOrder(t *testing.T) {
	fsys := fstest.MapFS{
		"i18n/en.toml": {Data: []byte(`yes = "Agree"`)},
	}

	// LoadFS first, then Merge — plugin's "Agree" should survive.
	cat := NewCatalog("en").
		LoadFS(fsys, "i18n").
		Merge(CommonMessages)

	if got := cat.T("en", "yes"); got != "Agree" {
		t.Errorf("T(en, yes) = %q, want Agree (not overwritten by Merge)", got)
	}
	// CommonMessages "no" should still be available.
	if got := cat.T("en", "no"); got != "No" {
		t.Errorf("T(en, no) = %q, want No", got)
	}
}

func TestCatalog_LoadFS_SkipsNonToml(t *testing.T) {
	fsys := fstest.MapFS{
		"i18n/en.toml":   {Data: []byte(`a = "A"`)},
		"i18n/readme.md": {Data: []byte(`# not a toml file`)},
	}

	cat := NewCatalog("en").LoadFS(fsys, "i18n")

	if got := cat.T("en", "a"); got != "A" {
		t.Errorf("T(en, a) = %q", got)
	}
	// Only "en" locale should be loaded.
	if len(cat.messages) != 1 {
		t.Errorf("expected 1 locale, got %d", len(cat.messages))
	}
}
