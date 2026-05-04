package plugin

import (
	"strings"

	"SuperBotGo/internal/model"
	"SuperBotGo/internal/state"
)

// ResolveCommand resolves user input to a unique command or returns multiple
// candidates when the alias is ambiguous.
//
// Input formats:
//   - Fully-qualified: "plugin_id.command_name" → direct lookup, no ambiguity.
//   - Short alias: "foo" → collected across all plugins.
//
// Returns:
//   - Single match: pluginID and def are set, candidates is nil.
//   - Collision: pluginID="" and def=nil, candidates lists all owners.
//   - Not found: all return values are zero/nil.
func (m *Manager) ResolveCommand(input string) (pluginID string, def *state.CommandDefinition, candidates []model.CommandCandidate) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Fully-qualified name: everything before the first dot is the plugin ID.
	if dotIdx := strings.IndexByte(input, '.'); dotIdx > 0 && dotIdx < len(input)-1 {
		pid := input[:dotIdx]
		cmdName := input[dotIdx+1:]
		if p, ok := m.plugins[pid]; ok {
			for _, d := range p.Commands() {
				if d.Name == cmdName {
					return pid, d, nil
				}
			}
		}
		return "", nil, nil
	}

	// Short alias — collect every plugin that owns this command name.
	type match struct {
		pluginID string
		def      *state.CommandDefinition
	}
	var matches []match

	for _, p := range m.plugins {
		for _, d := range p.Commands() {
			if d.Name == input {
				matches = append(matches, match{pluginID: p.ID(), def: d})
			}
		}
	}

	switch len(matches) {
	case 0:
		return "", nil, nil
	case 1:
		return matches[0].pluginID, matches[0].def, nil
	default:
		candidates = make([]model.CommandCandidate, len(matches))
		for i, mt := range matches {
			candidates[i] = model.CommandCandidate{
				PluginID:     mt.pluginID,
				CommandName:  mt.def.Name,
				FQName:       mt.pluginID + "." + mt.def.Name,
				Descriptions: copyStringMap(mt.def.Descriptions),
				Description:  mt.def.Description,
			}
		}
		return "", nil, candidates
	}
}
