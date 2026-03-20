package plugin

import (
	"sync"

	"SuperBotGo/internal/state"
)

type Manager struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
}

func NewManager() *Manager {
	return &Manager{
		plugins: make(map[string]Plugin),
	}
}

func (m *Manager) Load(plugins []Plugin) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range plugins {
		m.plugins[p.ID()] = p
	}
}

func (m *Manager) Register(p Plugin) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plugins[p.ID()] = p
}

func (m *Manager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.plugins, id)
}

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

func (m *Manager) GetPluginIDByCommand(commandName string) string {
	p := m.GetByCommand(commandName)
	if p == nil {
		return ""
	}
	return p.ID()
}

func (m *Manager) All() map[string]Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]Plugin, len(m.plugins))
	for k, v := range m.plugins {
		result[k] = v
	}
	return result
}
