package plugin

import (
	"reflect"
	"sync"
	"testing"

	"SuperBotGo/internal/state"
)

func newTestPlugin(id string, cmds ...string) *mockPlugin {
	defs := make([]*state.CommandDefinition, len(cmds))
	for i, c := range cmds {
		defs[i] = &state.CommandDefinition{Name: c, Description: c + " description"}
	}
	return &mockPlugin{id: id, name: id + "-name", version: "1.0.0", commands: defs}
}

func TestManager_RegisterAndGet(t *testing.T) {
	t.Parallel()

	mgr := NewManager()
	p := newTestPlugin("alpha", "cmd1")

	mgr.Register(p)

	got, ok := mgr.Get("alpha")
	if !ok {
		t.Fatal("expected ok=true after Register")
	}
	if got.ID() != "alpha" {
		t.Errorf("got.ID() = %q, want %q", got.ID(), "alpha")
	}
}

func TestManager_RemoveAndGet(t *testing.T) {
	t.Parallel()

	mgr := NewManager()
	mgr.Register(newTestPlugin("beta", "cmd1"))

	mgr.Remove("beta")

	_, ok := mgr.Get("beta")
	if ok {
		t.Error("expected ok=false after Remove")
	}
}

func TestManager_RemoveNonExistent(t *testing.T) {
	t.Parallel()

	mgr := NewManager()
	// Should not panic.
	mgr.Remove("nonexistent")
}

func TestManager_GetByCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cmd     string
		wantNil bool
		wantID  string
	}{
		{name: "found", cmd: "greet", wantID: "hello"},
		{name: "not found", cmd: "missing", wantNil: true},
	}

	mgr := NewManager()
	mgr.Register(newTestPlugin("hello", "greet", "wave"))

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p := mgr.GetByCommand(tc.cmd)
			if tc.wantNil {
				if p != nil {
					t.Errorf("expected nil plugin, got %q", p.ID())
				}
				return
			}
			if p == nil {
				t.Fatal("expected non-nil plugin, got nil")
			}
			if p.ID() != tc.wantID {
				t.Errorf("plugin ID = %q, want %q", p.ID(), tc.wantID)
			}
		})
	}
}

func TestManager_GetPluginIDByCommand(t *testing.T) {
	t.Parallel()

	mgr := NewManager()
	mgr.Register(newTestPlugin("foo", "bar"))

	tests := []struct {
		name string
		cmd  string
		want string
	}{
		{name: "existing command", cmd: "bar", want: "foo"},
		{name: "missing command", cmd: "baz", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := mgr.GetPluginIDByCommand(tc.cmd)
			if got != tc.want {
				t.Errorf("GetPluginIDByCommand(%q) = %q, want %q", tc.cmd, got, tc.want)
			}
		})
	}
}

func TestManager_GetCommandDefinition(t *testing.T) {
	t.Parallel()

	mgr := NewManager()
	mgr.Register(newTestPlugin("p1", "deploy", "rollback"))

	tests := []struct {
		name    string
		cmd     string
		wantNil bool
	}{
		{name: "found", cmd: "deploy"},
		{name: "not found", cmd: "destroy", wantNil: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			def := mgr.GetCommandDefinition(tc.cmd)
			if tc.wantNil {
				if def != nil {
					t.Errorf("expected nil, got %+v", def)
				}
				return
			}
			if def == nil {
				t.Fatal("expected non-nil definition")
			}
			if def.Name != tc.cmd {
				t.Errorf("def.Name = %q, want %q", def.Name, tc.cmd)
			}
		})
	}
}

func TestManager_All_ReturnsCopy(t *testing.T) {
	t.Parallel()

	mgr := NewManager()
	mgr.Register(newTestPlugin("x"))
	mgr.Register(newTestPlugin("y"))

	all := mgr.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(all))
	}

	// Mutating the returned map must not affect the manager's internal state.
	delete(all, "x")

	all2 := mgr.All()
	if len(all2) != 2 {
		t.Errorf("internal map was mutated: expected 2, got %d", len(all2))
	}
}

func TestManager_ListUserPluginsCarriesLocalizedDescriptions(t *testing.T) {
	t.Parallel()

	mgr := NewManager()
	mgr.Register(&mockPlugin{
		id:      "demo",
		name:    "Demo",
		version: "1.0.0",
		commands: []*state.CommandDefinition{
			{
				Name: "hello",
				Descriptions: map[string]string{
					"en": "Say hello",
					"ru": "Поздороваться",
				},
				Description: "Say hello",
			},
		},
	})

	plugins := mgr.ListUserPlugins()
	if len(plugins) != 1 {
		t.Fatalf("ListUserPlugins() len = %d, want 1", len(plugins))
	}
	if len(plugins[0].Commands) != 1 {
		t.Fatalf("commands len = %d, want 1", len(plugins[0].Commands))
	}
	want := map[string]string{"en": "Say hello", "ru": "Поздороваться"}
	if !reflect.DeepEqual(plugins[0].Commands[0].Descriptions, want) {
		t.Errorf("Descriptions = %#v, want %#v", plugins[0].Commands[0].Descriptions, want)
	}
	if plugins[0].Commands[0].Description != "Say hello" {
		t.Errorf("Description = %q, want %q", plugins[0].Commands[0].Description, "Say hello")
	}
}

func TestManager_Load(t *testing.T) {
	t.Parallel()

	mgr := NewManager()
	plugins := []Plugin{
		newTestPlugin("a", "a1"),
		newTestPlugin("b", "b1"),
		newTestPlugin("c", "c1"),
	}
	mgr.Load(plugins)

	all := mgr.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 plugins after Load, got %d", len(all))
	}
	for _, id := range []string{"a", "b", "c"} {
		if _, ok := all[id]; !ok {
			t.Errorf("plugin %q missing after Load", id)
		}
	}
}

func TestManager_ConcurrentRegisterGet(t *testing.T) {
	t.Parallel()

	mgr := NewManager()

	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Half the goroutines register, the other half read.
	for i := range goroutines {
		id := "plugin-" + string(rune('A'+i%26))
		go func() {
			defer wg.Done()
			mgr.Register(newTestPlugin(id, "cmd"))
		}()
		go func() {
			defer wg.Done()
			mgr.Get(id)
			mgr.All()
		}()
	}

	wg.Wait()

	// Verify the manager is still usable after concurrent access.
	all := mgr.All()
	if len(all) == 0 {
		t.Error("expected at least 1 plugin after concurrent registration")
	}
}
