package state

import (
	"testing"
)

func TestManager_UnregisterCommand(t *testing.T) {
	t.Parallel()

	m := NewManager(nil)
	m.RegisterCommand("plugin-a", &CommandDefinition{Name: "greet"})
	m.RegisterCommand("plugin-a", &CommandDefinition{Name: "wave"})

	m.UnregisterCommand("plugin-a", "greet")

	if _, ok := m.commands[fqKey("plugin-a", "greet")]; ok {
		t.Error("commands[greet] should be gone after UnregisterCommand")
	}
	if _, ok := m.handlers[fqKey("plugin-a", "greet")]; ok {
		t.Error("handlers[greet] should be gone after UnregisterCommand")
	}
	if _, ok := m.commands[fqKey("plugin-a", "wave")]; !ok {
		t.Error("commands[wave] should remain after unregistering a sibling")
	}
}

func TestManager_UnregisterAllCommands(t *testing.T) {
	t.Parallel()

	m := NewManager(nil)
	m.RegisterCommand("plugin-a", &CommandDefinition{Name: "greet"})
	m.RegisterCommand("plugin-a", &CommandDefinition{Name: "wave"})
	m.RegisterCommand("plugin-b", &CommandDefinition{Name: "ping"})

	m.UnregisterAllCommands("plugin-a")

	for _, name := range []string{"greet", "wave"} {
		key := fqKey("plugin-a", name)
		if _, ok := m.commands[key]; ok {
			t.Errorf("plugin-a.%s should be gone from commands", name)
		}
		if _, ok := m.handlers[key]; ok {
			t.Errorf("plugin-a.%s should be gone from handlers", name)
		}
	}

	if _, ok := m.commands[fqKey("plugin-b", "ping")]; !ok {
		t.Error("plugin-b.ping should remain — UnregisterAllCommands must not touch other plugins")
	}
	if _, ok := m.handlers[fqKey("plugin-b", "ping")]; !ok {
		t.Error("plugin-b.ping handler should remain")
	}
}

func TestManager_UnregisterAllCommands_UnknownPlugin(t *testing.T) {
	t.Parallel()

	m := NewManager(nil)
	m.RegisterCommand("plugin-a", &CommandDefinition{Name: "greet"})

	// Must be a no-op, not a panic.
	m.UnregisterAllCommands("plugin-missing")

	if _, ok := m.commands[fqKey("plugin-a", "greet")]; !ok {
		t.Error("plugin-a.greet should still be registered")
	}
}

func TestManager_UnregisterAllCommands_DoesNotMatchByOverlappingPrefix(t *testing.T) {
	t.Parallel()

	// "plugin" is a prefix of "plugin-a", but without the trailing dot
	// UnregisterAllCommands("plugin") must not match "plugin-a.cmd".
	m := NewManager(nil)
	m.RegisterCommand("plugin", &CommandDefinition{Name: "cmd"})
	m.RegisterCommand("plugin-a", &CommandDefinition{Name: "cmd"})

	m.UnregisterAllCommands("plugin")

	if _, ok := m.commands[fqKey("plugin", "cmd")]; ok {
		t.Error("plugin.cmd should be removed")
	}
	if _, ok := m.commands[fqKey("plugin-a", "cmd")]; !ok {
		t.Error("plugin-a.cmd must not be removed by UnregisterAllCommands(\"plugin\")")
	}
}
