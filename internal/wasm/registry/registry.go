package registry

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type PluginEntry struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Versions     []VersionEntry `json:"versions"`
	Dependencies []Dependency   `json:"dependencies,omitempty"`
	Signature    string         `json:"signature,omitempty"`
}

type VersionEntry struct {
	Version       string    `json:"version"`
	WasmHash      string    `json:"wasm_hash"`
	UploadedAt    time.Time `json:"uploaded_at"`
	Changelog     string    `json:"changelog,omitempty"`
	MinSDKVersion int       `json:"min_sdk_version,omitempty"`
}

type Dependency struct {
	PluginID          string `json:"plugin"`
	VersionConstraint string `json:"version"`
}

type PluginRegistry struct {
	mu      sync.RWMutex
	entries map[string]*PluginEntry
}

func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		entries: make(map[string]*PluginEntry),
	}
}

func (r *PluginRegistry) Register(entry PluginEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.entries[entry.ID]
	if !ok {
		cp := entry
		cp.Versions = make([]VersionEntry, len(entry.Versions))
		copy(cp.Versions, entry.Versions)
		r.entries[entry.ID] = &cp
		r.sortVersions(&cp)
		return
	}

	existing.Name = entry.Name
	existing.Dependencies = entry.Dependencies
	existing.Signature = entry.Signature

	for _, v := range entry.Versions {
		found := false
		for i, ev := range existing.Versions {
			if ev.Version == v.Version {
				existing.Versions[i] = v
				found = true
				break
			}
		}
		if !found {
			existing.Versions = append(existing.Versions, v)
		}
	}
	r.sortVersions(existing)
}

func (r *PluginRegistry) sortVersions(entry *PluginEntry) {
	sort.Slice(entry.Versions, func(i, j int) bool {
		return CompareVersions(entry.Versions[i].Version, entry.Versions[j].Version) > 0
	})
}

func (r *PluginRegistry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, id)
}

func (r *PluginRegistry) Get(id string) (PluginEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	e, ok := r.entries[id]
	if !ok {
		return PluginEntry{}, fmt.Errorf("plugin %q not found in registry", id)
	}
	return *e, nil
}

func (r *PluginRegistry) GetVersion(id, version string) (VersionEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	e, ok := r.entries[id]
	if !ok {
		return VersionEntry{}, fmt.Errorf("plugin %q not found in registry", id)
	}
	for _, v := range e.Versions {
		if v.Version == version {
			return v, nil
		}
	}
	return VersionEntry{}, fmt.Errorf("version %q not found for plugin %q", version, id)
}

func (r *PluginRegistry) GetLatest(id string) (VersionEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	e, ok := r.entries[id]
	if !ok {
		return VersionEntry{}, fmt.Errorf("plugin %q not found in registry", id)
	}
	if len(e.Versions) == 0 {
		return VersionEntry{}, fmt.Errorf("plugin %q has no versions", id)
	}
	return e.Versions[0], nil
}

func (r *PluginRegistry) ListAll() []PluginEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]PluginEntry, 0, len(r.entries))
	for _, e := range r.entries {
		result = append(result, *e)
	}
	return result
}

func (r *PluginRegistry) ListVersions(id string) ([]VersionEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	e, ok := r.entries[id]
	if !ok {
		return nil, fmt.Errorf("plugin %q not found in registry", id)
	}
	out := make([]VersionEntry, len(e.Versions))
	copy(out, e.Versions)
	return out, nil
}

func (r *PluginRegistry) Has(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.entries[id]
	return ok
}

func (r *PluginRegistry) HasVersion(id, version string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[id]
	if !ok {
		return false
	}
	for _, v := range e.Versions {
		if v.Version == version {
			return true
		}
	}
	return false
}
