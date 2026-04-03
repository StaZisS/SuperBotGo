package plugin

import (
	"context"
	"testing"

	"SuperBotGo/internal/model"
	"SuperBotGo/internal/state"
)

// mockPlugin is a minimal Plugin implementation for tests.
type mockPlugin struct {
	id       string
	name     string
	version  string
	commands []*state.CommandDefinition
}

func (m *mockPlugin) ID() string                           { return m.id }
func (m *mockPlugin) Name() string                         { return m.name }
func (m *mockPlugin) Version() string                      { return m.version }
func (m *mockPlugin) Commands() []*state.CommandDefinition { return m.commands }
func (m *mockPlugin) HandleEvent(_ context.Context, _ model.Event) (*model.EventResponse, error) {
	return nil, nil
}

func TestResolveCommand(t *testing.T) {
	t.Parallel()

	// Setup: two plugins, pluginA has "cmdX" and "shared",
	// pluginB has "cmdY" and "shared".
	pluginA := &mockPlugin{
		id:      "pluginA",
		name:    "Plugin A",
		version: "1.0.0",
		commands: []*state.CommandDefinition{
			{Name: "cmdX", Description: "command X from A"},
			{Name: "shared", Description: "shared from A"},
		},
	}
	pluginB := &mockPlugin{
		id:      "pluginB",
		name:    "Plugin B",
		version: "2.0.0",
		commands: []*state.CommandDefinition{
			{Name: "cmdY", Description: "command Y from B"},
			{Name: "shared", Description: "shared from B"},
		},
	}

	mgr := NewManager()
	mgr.Register(pluginA)
	mgr.Register(pluginB)

	tests := []struct {
		name           string
		input          string
		wantPluginID   string
		wantDef        bool // true if we expect a non-nil def
		wantDefName    string
		wantCandidates int // expected number of candidates
	}{
		{
			name:         "FQ exact match",
			input:        "pluginA.cmdX",
			wantPluginID: "pluginA",
			wantDef:      true,
			wantDefName:  "cmdX",
		},
		{
			name:         "FQ exact match different plugin",
			input:        "pluginB.cmdY",
			wantPluginID: "pluginB",
			wantDef:      true,
			wantDefName:  "cmdY",
		},
		{
			name:  "FQ non-existent plugin",
			input: "noSuchPlugin.cmdX",
		},
		{
			name:  "FQ non-existent command in existing plugin",
			input: "pluginA.noSuchCmd",
		},
		{
			name:         "short alias unique",
			input:        "cmdX",
			wantPluginID: "pluginA",
			wantDef:      true,
			wantDefName:  "cmdX",
		},
		{
			name:           "short alias collision",
			input:          "shared",
			wantCandidates: 2,
		},
		{
			name:  "short alias not found",
			input: "nonexistent",
		},
		{
			name:  "empty string",
			input: "",
		},
		{
			name:  "dot at start",
			input: ".cmdX",
		},
		{
			name:  "dot at end",
			input: "pluginA.",
		},
		{
			name:  "only a dot",
			input: ".",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pluginID, def, candidates := mgr.ResolveCommand(tc.input)

			if pluginID != tc.wantPluginID {
				t.Errorf("pluginID = %q, want %q", pluginID, tc.wantPluginID)
			}

			if tc.wantDef {
				if def == nil {
					t.Fatal("expected non-nil def, got nil")
				}
				if def.Name != tc.wantDefName {
					t.Errorf("def.Name = %q, want %q", def.Name, tc.wantDefName)
				}
			} else {
				if def != nil {
					t.Errorf("expected nil def, got %+v", def)
				}
			}

			if len(candidates) != tc.wantCandidates {
				t.Errorf("len(candidates) = %d, want %d", len(candidates), tc.wantCandidates)
			}

			// When we have candidates, verify their FQNames are well-formed.
			for _, c := range candidates {
				if c.FQName == "" {
					t.Error("candidate FQName is empty")
				}
				if c.PluginID == "" {
					t.Error("candidate PluginID is empty")
				}
				if c.CommandName == "" {
					t.Error("candidate CommandName is empty")
				}
			}
		})
	}
}

func TestResolveCommand_CollisionFQNames(t *testing.T) {
	t.Parallel()

	pluginA := &mockPlugin{
		id:       "pluginA",
		commands: []*state.CommandDefinition{{Name: "dup", Description: "dup A"}},
	}
	pluginB := &mockPlugin{
		id:       "pluginB",
		commands: []*state.CommandDefinition{{Name: "dup", Description: "dup B"}},
	}

	mgr := NewManager()
	mgr.Register(pluginA)
	mgr.Register(pluginB)

	_, _, candidates := mgr.ResolveCommand("dup")
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}

	fqNames := map[string]bool{}
	for _, c := range candidates {
		fqNames[c.FQName] = true
	}
	if !fqNames["pluginA.dup"] {
		t.Error("missing candidate pluginA.dup")
	}
	if !fqNames["pluginB.dup"] {
		t.Error("missing candidate pluginB.dup")
	}
}
