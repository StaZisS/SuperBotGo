package wasmplugin

import (
	"testing"
)

func TestCatalog_L_AllLocales(t *testing.T) {
	cat := NewCatalog("en").
		Add("en", map[string]string{"hello": "Hello"}).
		Add("ru", map[string]string{"hello": "Привет"})

	got := cat.L("hello")
	if got["en"] != "Hello" {
		t.Errorf("L(hello)[en] = %q, want Hello", got["en"])
	}
	if got["ru"] != "Привет" {
		t.Errorf("L(hello)[ru] = %q, want Привет", got["ru"])
	}
}

func TestCatalog_L_MissingKey(t *testing.T) {
	cat := NewCatalog("en").
		Add("en", map[string]string{"hello": "Hello"})

	got := cat.L("missing")
	if got["en"] != "missing" {
		t.Errorf("L(missing)[en] = %q, want 'missing' (key as fallback)", got["en"])
	}
}

func TestCatalog_L_NamedArgs(t *testing.T) {
	cat := NewCatalog("en").
		Add("en", map[string]string{"greet": "Hello, {{.Name}}!"}).
		Add("ru", map[string]string{"greet": "Привет, {{.Name}}!"})

	got := cat.L("greet", "Name", "Alice")
	if got["en"] != "Hello, Alice!" {
		t.Errorf("L(greet)[en] = %q, want 'Hello, Alice!'", got["en"])
	}
	if got["ru"] != "Привет, Alice!" {
		t.Errorf("L(greet)[ru] = %q, want 'Привет, Alice!'", got["ru"])
	}
}

func TestCatalog_L_PositionalArgs(t *testing.T) {
	cat := NewCatalog("en").
		Add("en", map[string]string{"msg": "{{.V0}} has {{.V1}} items"})

	got := cat.L("msg", 42, "five")
	if got["en"] != "42 has five items" {
		t.Errorf("L(msg)[en] = %q, want '42 has five items'", got["en"])
	}
}

func TestCatalog_T_ExactMatch(t *testing.T) {
	cat := NewCatalog("en").
		Add("en", map[string]string{"ok": "OK"}).
		Add("ru", map[string]string{"ok": "ОК"})

	if got := cat.T("ru", "ok"); got != "ОК" {
		t.Errorf("T(ru, ok) = %q, want ОК", got)
	}
}

func TestCatalog_T_PrefixFallback(t *testing.T) {
	cat := NewCatalog("en").
		Add("ru", map[string]string{"ok": "ОК"})

	if got := cat.T("ru-RU", "ok"); got != "ОК" {
		t.Errorf("T(ru-RU, ok) = %q, want ОК (prefix fallback)", got)
	}
}

func TestCatalog_T_DefaultLocaleFallback(t *testing.T) {
	cat := NewCatalog("en").
		Add("en", map[string]string{"ok": "OK"})

	if got := cat.T("fr", "ok"); got != "OK" {
		t.Errorf("T(fr, ok) = %q, want OK (default locale fallback)", got)
	}
}

func TestCatalog_T_KeyFallback(t *testing.T) {
	cat := NewCatalog("en")

	if got := cat.T("en", "missing_key"); got != "missing_key" {
		t.Errorf("T(en, missing_key) = %q, want missing_key", got)
	}
}

func TestCatalog_T_WithArgs(t *testing.T) {
	cat := NewCatalog("en").
		Add("en", map[string]string{"hi": "Hi, {{.Name}}!"})

	if got := cat.T("en", "hi", "Name", "Bob"); got != "Hi, Bob!" {
		t.Errorf("T(en, hi) = %q, want 'Hi, Bob!'", got)
	}
}

func TestCatalog_Tr(t *testing.T) {
	cat := NewCatalog("en").
		Add("en", map[string]string{"a": "A", "b": "B"}).
		Add("ru", map[string]string{"a": "А", "b": "Б"})

	tr := cat.Tr("ru")
	if got := tr("a"); got != "А" {
		t.Errorf("tr(a) = %q, want А", got)
	}
	if got := tr("b"); got != "Б" {
		t.Errorf("tr(b) = %q, want Б", got)
	}
}

func TestCatalog_Opt(t *testing.T) {
	cat := NewCatalog("en").
		Add("en", map[string]string{"yes": "Yes"}).
		Add("ru", map[string]string{"yes": "Да"})

	opt := cat.Opt("yes", "true")
	if opt.Value != "true" {
		t.Errorf("Opt.Value = %q, want true", opt.Value)
	}
	if opt.Label != "Yes" {
		t.Errorf("Opt.Label = %q, want Yes (default locale)", opt.Label)
	}
	if opt.Labels["en"] != "Yes" {
		t.Errorf("Opt.Labels[en] = %q, want Yes", opt.Labels["en"])
	}
	if opt.Labels["ru"] != "Да" {
		t.Errorf("Opt.Labels[ru] = %q, want Да", opt.Labels["ru"])
	}
}

func TestCatalog_Merge_NoOverwrite(t *testing.T) {
	base := NewCatalog("en").
		Add("en", map[string]string{"yes": "Agree"})

	other := NewCatalog("en").
		Add("en", map[string]string{"yes": "Yes", "no": "No"})

	base.Merge(other)

	if got := base.T("en", "yes"); got != "Agree" {
		t.Errorf("after Merge, T(en, yes) = %q, want Agree (not overwritten)", got)
	}
	// But new keys from other are available.
	if got := base.T("en", "no"); got != "No" {
		t.Errorf("after Merge, T(en, no) = %q, want No (from other)", got)
	}
}

func TestCatalog_Merge_AddsNewLocales(t *testing.T) {
	cat := NewCatalog("en").
		Add("en", map[string]string{"hello": "Hello"})

	other := NewCatalog("en").
		Add("ru", map[string]string{"hello": "Привет"})

	cat.Merge(other)

	got := cat.L("hello")
	if got["ru"] != "Привет" {
		t.Errorf("after Merge, L(hello)[ru] = %q, want Привет", got["ru"])
	}
}

func TestCatalog_Add_OverwritesExisting(t *testing.T) {
	cat := NewCatalog("en").
		Add("en", map[string]string{"ok": "OK"}).
		Add("en", map[string]string{"ok": "Okay"})

	if got := cat.T("en", "ok"); got != "Okay" {
		t.Errorf("T(en, ok) = %q, want Okay (overwritten by second Add)", got)
	}
}

func TestInterpolate_MultipleNamedArgs(t *testing.T) {
	cat := NewCatalog("en").
		Add("en", map[string]string{
			"header": "{{.Schedule}}\n{{.Building}} {{.Bld}}, {{.Room}} {{.Rm}}",
		})

	got := cat.T("en", "header", "Schedule", "Schedule", "Building", "Building", "Bld", "1", "Room", "Room", "Rm", "203")
	want := "Schedule\nBuilding 1, Room 203"
	if got != want {
		t.Errorf("T = %q, want %q", got, want)
	}
}
