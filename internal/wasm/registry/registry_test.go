package registry

import (
	"testing"
	"time"
)

func TestRegisterAndGet(t *testing.T) {
	r := NewPluginRegistry()

	entry := PluginEntry{
		ID:   "test-plugin",
		Name: "Test Plugin",
		Versions: []VersionEntry{
			{Version: "1.0.0", WasmHash: "abc123", UploadedAt: time.Now()},
		},
	}
	r.Register(entry)

	got, err := r.Get("test-plugin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "test-plugin" {
		t.Errorf("got ID %q, want %q", got.ID, "test-plugin")
	}
	if got.Name != "Test Plugin" {
		t.Errorf("got Name %q, want %q", got.Name, "Test Plugin")
	}
	if len(got.Versions) != 1 {
		t.Fatalf("got %d versions, want 1", len(got.Versions))
	}
}

func TestGetNotFound(t *testing.T) {
	r := NewPluginRegistry()
	_, err := r.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent plugin")
	}
}

func TestRegisterMergesVersions(t *testing.T) {
	r := NewPluginRegistry()

	r.Register(PluginEntry{
		ID:   "p1",
		Name: "Plugin",
		Versions: []VersionEntry{
			{Version: "1.0.0", WasmHash: "h1"},
		},
	})

	r.Register(PluginEntry{
		ID:   "p1",
		Name: "Plugin Updated",
		Versions: []VersionEntry{
			{Version: "2.0.0", WasmHash: "h2"},
		},
	})

	e, err := r.Get("p1")
	if err != nil {
		t.Fatal(err)
	}
	if e.Name != "Plugin Updated" {
		t.Errorf("name not updated: got %q", e.Name)
	}
	if len(e.Versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(e.Versions))
	}
	// Latest should be first.
	if e.Versions[0].Version != "2.0.0" {
		t.Errorf("expected latest version first, got %q", e.Versions[0].Version)
	}
}

func TestRegisterUpdatesExistingVersion(t *testing.T) {
	r := NewPluginRegistry()

	r.Register(PluginEntry{
		ID:       "p1",
		Name:     "Plugin",
		Versions: []VersionEntry{{Version: "1.0.0", WasmHash: "old"}},
	})
	r.Register(PluginEntry{
		ID:       "p1",
		Name:     "Plugin",
		Versions: []VersionEntry{{Version: "1.0.0", WasmHash: "new"}},
	})

	e, _ := r.Get("p1")
	if len(e.Versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(e.Versions))
	}
	if e.Versions[0].WasmHash != "new" {
		t.Errorf("hash not updated: got %q", e.Versions[0].WasmHash)
	}
}

func TestGetVersion(t *testing.T) {
	r := NewPluginRegistry()
	r.Register(PluginEntry{
		ID: "p1",
		Versions: []VersionEntry{
			{Version: "1.0.0", WasmHash: "h1"},
			{Version: "2.0.0", WasmHash: "h2"},
		},
	})

	v, err := r.GetVersion("p1", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if v.WasmHash != "h1" {
		t.Errorf("got hash %q, want %q", v.WasmHash, "h1")
	}

	_, err = r.GetVersion("p1", "3.0.0")
	if err == nil {
		t.Fatal("expected error for missing version")
	}
}

func TestGetLatest(t *testing.T) {
	r := NewPluginRegistry()
	r.Register(PluginEntry{
		ID: "p1",
		Versions: []VersionEntry{
			{Version: "1.0.0"},
			{Version: "3.1.0"},
			{Version: "2.5.0"},
		},
	})

	v, err := r.GetLatest("p1")
	if err != nil {
		t.Fatal(err)
	}
	if v.Version != "3.1.0" {
		t.Errorf("got latest %q, want %q", v.Version, "3.1.0")
	}
}

func TestGetLatestEmpty(t *testing.T) {
	r := NewPluginRegistry()
	r.Register(PluginEntry{ID: "empty"})
	_, err := r.GetLatest("empty")
	if err == nil {
		t.Fatal("expected error for plugin with no versions")
	}
}

func TestListAll(t *testing.T) {
	r := NewPluginRegistry()
	r.Register(PluginEntry{ID: "a", Name: "A"})
	r.Register(PluginEntry{ID: "b", Name: "B"})

	all := r.ListAll()
	if len(all) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(all))
	}
}

func TestListVersions(t *testing.T) {
	r := NewPluginRegistry()
	r.Register(PluginEntry{
		ID: "p1",
		Versions: []VersionEntry{
			{Version: "1.0.0"},
			{Version: "2.0.0"},
		},
	})

	vv, err := r.ListVersions("p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(vv) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(vv))
	}
}

func TestRemove(t *testing.T) {
	r := NewPluginRegistry()
	r.Register(PluginEntry{ID: "p1"})
	r.Remove("p1")

	if r.Has("p1") {
		t.Error("plugin should have been removed")
	}
}

func TestHas(t *testing.T) {
	r := NewPluginRegistry()
	r.Register(PluginEntry{ID: "p1"})

	if !r.Has("p1") {
		t.Error("expected Has to return true")
	}
	if r.Has("nope") {
		t.Error("expected Has to return false")
	}
}

func TestHasVersion(t *testing.T) {
	r := NewPluginRegistry()
	r.Register(PluginEntry{
		ID:       "p1",
		Versions: []VersionEntry{{Version: "1.0.0"}},
	})

	if !r.HasVersion("p1", "1.0.0") {
		t.Error("expected HasVersion to return true")
	}
	if r.HasVersion("p1", "9.9.9") {
		t.Error("expected HasVersion to return false for missing version")
	}
	if r.HasVersion("nope", "1.0.0") {
		t.Error("expected HasVersion to return false for missing plugin")
	}
}
