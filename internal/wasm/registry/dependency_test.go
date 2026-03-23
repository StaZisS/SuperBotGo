package registry

import (
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"2.0.0", "1.9.9", 1},
		{"1.9.9", "2.0.0", -1},
		{"1.0", "1.0.0", 0},
		{"1", "1.0.0", 0},
		{"10.0.0", "9.0.0", 1},
		{"0.1.0", "0.0.1", 1},
	}

	for _, tt := range tests {
		got := CompareVersions(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestSatisfiesConstraint(t *testing.T) {
	tests := []struct {
		version    string
		constraint string
		want       bool
	}{
		// Greater than or equal.
		{"2.0.0", ">=2.0.0", true},
		{"2.0.1", ">=2.0.0", true},
		{"1.9.0", ">=2.0.0", false},

		// Strictly greater.
		{"2.0.1", ">2.0.0", true},
		{"2.0.0", ">2.0.0", false},

		// Less than or equal.
		{"2.0.0", "<=2.0.0", true},
		{"1.9.0", "<=2.0.0", true},
		{"2.0.1", "<=2.0.0", false},

		// Strictly less.
		{"1.9.0", "<2.0.0", true},
		{"2.0.0", "<2.0.0", false},

		// Exact match.
		{"2.0.0", "=2.0.0", true},
		{"2.0.0", "==2.0.0", true},
		{"2.0.0", "2.0.0", true},
		{"2.0.1", "=2.0.0", false},

		// Not equal.
		{"2.0.1", "!=2.0.0", true},
		{"2.0.0", "!=2.0.0", false},

		// Caret (compatible).
		{"1.5.0", "^1.2.0", true},
		{"1.2.0", "^1.2.0", true},
		{"1.1.0", "^1.2.0", false},
		{"2.0.0", "^1.2.0", false},

		// Tilde (approximately).
		{"1.2.5", "~1.2.0", true},
		{"1.2.0", "~1.2.0", true},
		{"1.3.0", "~1.2.0", false},
		{"1.1.0", "~1.2.0", false},

		// Wildcard / empty.
		{"1.0.0", "*", true},
		{"1.0.0", "", true},
	}

	for _, tt := range tests {
		got := SatisfiesConstraint(tt.version, tt.constraint)
		if got != tt.want {
			t.Errorf("SatisfiesConstraint(%q, %q) = %v, want %v", tt.version, tt.constraint, got, tt.want)
		}
	}
}

func TestResolveDependencies_AllMet(t *testing.T) {
	r := NewPluginRegistry()
	r.Register(PluginEntry{
		ID:   "plugin-a",
		Name: "Plugin A",
		Dependencies: []Dependency{
			{PluginID: "plugin-b", VersionConstraint: ">=1.0.0"},
		},
		Versions: []VersionEntry{{Version: "1.0.0"}},
	})

	installed := []InstalledPlugin{
		{ID: "plugin-b", Version: "2.0.0"},
	}

	err := ResolveDependencies(r, "plugin-a", "1.0.0", installed)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestResolveDependencies_NotInstalled(t *testing.T) {
	r := NewPluginRegistry()
	r.Register(PluginEntry{
		ID: "plugin-a",
		Dependencies: []Dependency{
			{PluginID: "plugin-b", VersionConstraint: ">=1.0.0"},
		},
		Versions: []VersionEntry{{Version: "1.0.0"}},
	})

	err := ResolveDependencies(r, "plugin-a", "1.0.0", nil)
	if err == nil {
		t.Fatal("expected error for missing dependency")
	}

	depErr, ok := err.(*DependencyError)
	if !ok {
		t.Fatalf("expected *DependencyError, got %T", err)
	}
	if len(depErr.Unmet) != 1 {
		t.Fatalf("expected 1 unmet, got %d", len(depErr.Unmet))
	}
	if depErr.Unmet[0].Reason != "not installed" {
		t.Errorf("expected reason 'not installed', got %q", depErr.Unmet[0].Reason)
	}
}

func TestResolveDependencies_VersionMismatch(t *testing.T) {
	r := NewPluginRegistry()
	r.Register(PluginEntry{
		ID: "plugin-a",
		Dependencies: []Dependency{
			{PluginID: "plugin-b", VersionConstraint: ">=3.0.0"},
		},
		Versions: []VersionEntry{{Version: "1.0.0"}},
	})

	installed := []InstalledPlugin{
		{ID: "plugin-b", Version: "2.0.0"},
	}

	err := ResolveDependencies(r, "plugin-a", "1.0.0", installed)
	if err == nil {
		t.Fatal("expected error for version mismatch")
	}

	depErr, ok := err.(*DependencyError)
	if !ok {
		t.Fatalf("expected *DependencyError, got %T", err)
	}
	if len(depErr.Unmet) != 1 {
		t.Fatalf("expected 1 unmet, got %d", len(depErr.Unmet))
	}
}

func TestResolveDependencies_NoDeps(t *testing.T) {
	r := NewPluginRegistry()
	r.Register(PluginEntry{
		ID:       "plugin-a",
		Versions: []VersionEntry{{Version: "1.0.0"}},
	})

	err := ResolveDependencies(r, "plugin-a", "1.0.0", nil)
	if err != nil {
		t.Fatalf("expected no error for plugin with no deps, got: %v", err)
	}
}

func TestResolveDependencies_PluginNotInRegistry(t *testing.T) {
	r := NewPluginRegistry()
	err := ResolveDependencies(r, "nonexistent", "1.0.0", nil)
	if err == nil {
		t.Fatal("expected error for plugin not in registry")
	}
}
