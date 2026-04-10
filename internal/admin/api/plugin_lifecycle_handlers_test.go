package api

import (
	"context"
	"testing"

	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/state"
)

// fakePlugin is a minimal plugin.Plugin implementation for handler tests.
// Only ID and Commands are meaningful; the rest satisfy the interface.
type fakePlugin struct {
	id   string
	cmds []*state.CommandDefinition
}

func (p *fakePlugin) ID() string                           { return p.id }
func (p *fakePlugin) Name() string                         { return p.id }
func (p *fakePlugin) Version() string                      { return "test" }
func (p *fakePlugin) Commands() []*state.CommandDefinition { return p.cmds }
func (p *fakePlugin) HandleEvent(_ context.Context, _ model.Event) (*model.EventResponse, error) {
	return nil, nil
}

func newFakePlugin(id string, cmdNames ...string) *fakePlugin {
	defs := make([]*state.CommandDefinition, len(cmdNames))
	for i, name := range cmdNames {
		defs[i] = &state.CommandDefinition{Name: name}
	}
	return &fakePlugin{id: id, cmds: defs}
}

// recordingStateMgr is a StateManagerRegistrar that records every call so
// tests can assert which code path was taken and with which arguments.
type recordingStateMgr struct {
	registered      []string
	unregistered    []string
	unregisteredAll []string
}

func (r *recordingStateMgr) RegisterCommand(pluginID string, def *state.CommandDefinition) {
	r.registered = append(r.registered, pluginID+"."+def.Name)
}

func (r *recordingStateMgr) UnregisterCommand(pluginID, name string) {
	r.unregistered = append(r.unregistered, pluginID+"."+name)
}

func (r *recordingStateMgr) UnregisterAllCommands(pluginID string) {
	r.unregisteredAll = append(r.unregisteredAll, pluginID)
}

// newTestAdminHandler constructs an AdminHandler populated only with the
// fields that unregisterPluginCommands touches. Calling any other handler
// method would panic — that is intentional.
func newTestAdminHandler(mgr *plugin.Manager, sm StateManagerRegistrar) *AdminHandler {
	return &AdminHandler{
		manager:  mgr,
		stateMgr: sm,
	}
}

func TestUnregisterPluginCommands_NilStateMgrIsNoop(t *testing.T) {
	t.Parallel()

	mgr := plugin.NewManager()
	mgr.Register(newFakePlugin("p1", "cmd1"))

	h := newTestAdminHandler(mgr, nil)
	h.unregisterPluginCommands("p1") // must not panic
}

// When the plugin is still in plugin.Manager (enabled path), we must iterate
// the plugin's own Commands() and call UnregisterCommand for each. We must
// NOT fall back to UnregisterAllCommands.
func TestUnregisterPluginCommands_EnabledPluginUsesPerCommandUnregister(t *testing.T) {
	t.Parallel()

	rec := &recordingStateMgr{}
	mgr := plugin.NewManager()
	mgr.Register(newFakePlugin("p1", "alpha", "beta"))

	h := newTestAdminHandler(mgr, rec)
	h.unregisterPluginCommands("p1")

	if len(rec.unregisteredAll) != 0 {
		t.Errorf("UnregisterAllCommands must not be called when plugin is in manager, got %v", rec.unregisteredAll)
	}

	want := map[string]bool{"p1.alpha": true, "p1.beta": true}
	if len(rec.unregistered) != len(want) {
		t.Fatalf("expected %d UnregisterCommand calls, got %d: %v", len(want), len(rec.unregistered), rec.unregistered)
	}
	for _, got := range rec.unregistered {
		if !want[got] {
			t.Errorf("unexpected UnregisterCommand call: %s", got)
		}
	}
}

// Core regression: a plugin that is not in plugin.Manager (disabled, or
// left in an inconsistent state) must still have its state-machine commands
// wiped. The helper must fall back to UnregisterAllCommands.
func TestUnregisterPluginCommands_UnknownPluginFallsBackToUnregisterAll(t *testing.T) {
	t.Parallel()

	rec := &recordingStateMgr{}
	mgr := plugin.NewManager()
	// Intentionally: p1 is NOT registered in plugin.Manager.

	h := newTestAdminHandler(mgr, rec)
	h.unregisterPluginCommands("p1")

	if len(rec.unregistered) != 0 {
		t.Errorf("no per-command Unregister expected for unknown plugin, got %v", rec.unregistered)
	}
	if len(rec.unregisteredAll) != 1 || rec.unregisteredAll[0] != "p1" {
		t.Errorf("expected UnregisterAllCommands(p1) exactly once, got %v", rec.unregisteredAll)
	}
}

// End-to-end integration with the real state.Manager: simulate a
// disable-then-delete cycle for a plugin whose commands were left in the
// state manager, and verify they are cleaned up.
func TestUnregisterPluginCommands_RealStateManager_DisabledPluginCleanup(t *testing.T) {
	t.Parallel()

	sm := state.NewManager(nil)
	sm.RegisterCommand("p1", &state.CommandDefinition{Name: "cmd1", PreservesDialog: true})
	sm.RegisterCommand("p1", &state.CommandDefinition{Name: "cmd2", PreservesDialog: true})
	sm.RegisterCommand("p2", &state.CommandDefinition{Name: "keep", PreservesDialog: true})

	// IsPreservesDialog is the cheapest public way to probe whether a
	// command is registered: it returns false for missing commands and
	// true for commands with PreservesDialog set.
	if !sm.IsPreservesDialog("p1", "cmd1") {
		t.Fatal("setup: p1.cmd1 should be registered")
	}

	mgr := plugin.NewManager()
	mgr.Register(newFakePlugin("p2", "keep"))
	// p1 is deliberately absent from plugin.Manager.

	h := newTestAdminHandler(mgr, sm)
	h.unregisterPluginCommands("p1")

	if sm.IsPreservesDialog("p1", "cmd1") {
		t.Error("p1.cmd1 should be gone after unregisterPluginCommands")
	}
	if sm.IsPreservesDialog("p1", "cmd2") {
		t.Error("p1.cmd2 should be gone after unregisterPluginCommands")
	}
	if !sm.IsPreservesDialog("p2", "keep") {
		t.Error("p2.keep must not be touched when cleaning up p1")
	}
}

// Integration: an enabled plugin's commands are removed from a real
// state.Manager when unregisterPluginCommands runs.
func TestUnregisterPluginCommands_RealStateManager_EnabledPluginCleanup(t *testing.T) {
	t.Parallel()

	sm := state.NewManager(nil)
	sm.RegisterCommand("p1", &state.CommandDefinition{Name: "cmd1", PreservesDialog: true})
	sm.RegisterCommand("p1", &state.CommandDefinition{Name: "cmd2", PreservesDialog: true})

	mgr := plugin.NewManager()
	mgr.Register(newFakePlugin("p1", "cmd1", "cmd2"))

	h := newTestAdminHandler(mgr, sm)
	h.unregisterPluginCommands("p1")

	if sm.IsPreservesDialog("p1", "cmd1") {
		t.Error("p1.cmd1 should be gone")
	}
	if sm.IsPreservesDialog("p1", "cmd2") {
		t.Error("p1.cmd2 should be gone")
	}
}
