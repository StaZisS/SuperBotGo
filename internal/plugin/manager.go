package plugin

import (
	"sync"

	"SuperBotGo/internal/state"
)

// Manager registers and looks up plugins and their commands.
type Manager struct {
	mu      sync.RWMutex
	plugins map[string]Plugin // keyed by plugin ID
}

// NewManager creates an empty PluginManager.
func NewManager() *Manager {
	return &Manager{
		plugins: make(map[string]Plugin),
	}
}

// Load registers all provided plugins.
func (m *Manager) Load(plugins []Plugin) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range plugins {
		m.plugins[p.ID()] = p
	}
}

// Register adds a single plugin.
func (m *Manager) Register(p Plugin) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plugins[p.ID()] = p
}

// Remove unregisters a plugin by ID.
func (m *Manager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.plugins, id)
}

// GetByCommand returns the plugin that handles the given command, or nil.
func (m *Manager) GetByCommand(commandName string) Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.plugins {
		for _, name := range CommandNames(p) {
			if name == commandName {
				return p
			}
		}
	}
	return nil
}

// GetCommandDefinition returns the command definition for the given command name,
// or nil if no plugin provides it.
func (m *Manager) GetCommandDefinition(commandName string) *state.CommandDefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.plugins {
		for _, def := range p.Commands() {
			if def.Name == commandName {
				return def
			}
		}
	}
	return nil
}

// GetPluginIDByCommand returns the plugin ID that handles the given command, or empty string.
func (m *Manager) GetPluginIDByCommand(commandName string) string {
	p := m.GetByCommand(commandName)
	if p == nil {
		return ""
	}
	return p.ID()
}

// All returns all registered plugins as a map of ID to Plugin.
func (m *Manager) All() map[string]Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]Plugin, len(m.plugins))
	for k, v := range m.plugins {
		result[k] = v
	}
	return result
}
