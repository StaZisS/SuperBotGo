package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/state"
	"SuperBotGo/internal/wasm/adapter"
	wasmrt "SuperBotGo/internal/wasm/runtime"
)

type testPluginStore struct {
	records  map[string]PluginRecord
	metadata map[string]PluginMetadataRecord
}

func (s *testPluginStore) SavePlugin(_ context.Context, record PluginRecord) error {
	if s.records == nil {
		s.records = make(map[string]PluginRecord)
	}
	s.records[record.ID] = record
	return nil
}

func (s *testPluginStore) GetPlugin(_ context.Context, id string) (PluginRecord, error) {
	rec, ok := s.records[id]
	if !ok {
		return PluginRecord{}, errNotFound(id)
	}
	return rec, nil
}

func (s *testPluginStore) ListPlugins(_ context.Context) ([]PluginRecord, error) {
	var out []PluginRecord
	for _, rec := range s.records {
		out = append(out, rec)
	}
	return out, nil
}

func (s *testPluginStore) DeletePlugin(_ context.Context, id string) error {
	delete(s.records, id)
	return nil
}

func (s *testPluginStore) SavePluginMetadata(_ context.Context, record PluginMetadataRecord) error {
	if s.metadata == nil {
		s.metadata = make(map[string]PluginMetadataRecord)
	}
	s.metadata[record.PluginID] = record
	return nil
}

func (s *testPluginStore) GetPluginMetadata(_ context.Context, id string) (PluginMetadataRecord, error) {
	rec, ok := s.metadata[id]
	if !ok {
		return PluginMetadataRecord{}, errNotFound(id)
	}
	return rec, nil
}

func (s *testPluginStore) DeletePluginMetadata(_ context.Context, id string) error {
	delete(s.metadata, id)
	return nil
}

func TestPluginLifecycleServiceValidateConfigUsesStoredMetadata(t *testing.T) {
	t.Parallel()

	store := &testPluginStore{
		metadata: map[string]PluginMetadataRecord{
			"demo": {
				PluginID:     "demo",
				ConfigSchema: json.RawMessage(`{"type":"object","properties":{"token":{"type":"string"}},"required":["token"]}`),
			},
		},
	}

	svc := NewPluginLifecycleService(
		store,
		nil,
		&adapter.Loader{},
		plugin.NewManager(),
		nil,
		nil,
		nil,
		nil,
		nil,
		PluginLifecycleOptions{ReconfigureEnabled: true},
	)

	if err := svc.ValidateConfig(context.Background(), "demo", json.RawMessage(`{"token":"secret"}`)); err != nil {
		t.Fatalf("ValidateConfig(valid) error = %v, want nil", err)
	}

	if err := svc.ValidateConfig(context.Background(), "demo", json.RawMessage(`{}`)); err == nil {
		t.Fatal("ValidateConfig(invalid) error = nil, want validation error")
	}
}

type notFoundError struct{ id string }

func (e notFoundError) Error() string { return "not found: " + e.id }

func errNotFound(id string) error { return notFoundError{id: id} }

type testLifecyclePlugin struct {
	id       string
	name     string
	version  string
	meta     wasmrt.PluginMeta
	commands []*state.CommandDefinition
}

func (p *testLifecyclePlugin) ID() string      { return p.id }
func (p *testLifecyclePlugin) Name() string    { return p.name }
func (p *testLifecyclePlugin) Version() string { return p.version }
func (p *testLifecyclePlugin) Commands() []*state.CommandDefinition {
	return p.commands
}
func (p *testLifecyclePlugin) HandleEvent(context.Context, model.Event) (*model.EventResponse, error) {
	return nil, nil
}

func (p *testLifecyclePlugin) Meta() wasmrt.PluginMeta {
	meta := p.meta
	if meta.ID == "" {
		meta.ID = p.id
	}
	if meta.Name == "" {
		meta.Name = p.name
	}
	if meta.Version == "" {
		meta.Version = p.version
	}
	return meta
}

type testLifecycleLoader struct {
	loadPlugin lifecyclePlugin
	loadErr    error
	reloadErr  error
	probeMeta  wasmrt.PluginMeta
	probeErr   error
	plugins    map[string]lifecyclePlugin
	unloaded   []string
}

func (l *testLifecycleLoader) GetPlugin(pluginID string) (lifecyclePlugin, bool) {
	p, ok := l.plugins[pluginID]
	return p, ok
}

func (l *testLifecycleLoader) LoadPluginFromBytes(context.Context, []byte, json.RawMessage) (lifecyclePlugin, error) {
	return l.loadPlugin, l.loadErr
}

func (l *testLifecycleLoader) ReloadPluginFromBytes(context.Context, string, []byte, json.RawMessage) error {
	return l.reloadErr
}

func (l *testLifecycleLoader) ProbeMetadataFromBytes(context.Context, []byte) (wasmrt.PluginMeta, error) {
	return l.probeMeta, l.probeErr
}

func (l *testLifecycleLoader) UnloadPlugin(_ context.Context, pluginID string) error {
	l.unloaded = append(l.unloaded, pluginID)
	return nil
}

func (l *testLifecycleLoader) ReconfigurePlugin(context.Context, string, json.RawMessage) error {
	return nil
}

func (l *testLifecycleLoader) DropPluginData(context.Context, string) error {
	return nil
}

func TestPluginLifecycleServiceLoadPluginBytesRejectsMismatchedID(t *testing.T) {
	t.Parallel()

	svc := NewPluginLifecycleService(
		&testPluginStore{},
		nil,
		&adapter.Loader{},
		plugin.NewManager(),
		nil,
		nil,
		nil,
		nil,
		nil,
		PluginLifecycleOptions{},
	)
	loader := &testLifecycleLoader{
		loadPlugin: &testLifecyclePlugin{id: "other", name: "Other", version: "1.0.0"},
	}
	svc.loader = loader

	_, err := svc.loadPluginBytes(context.Background(), "demo", []byte("wasm"), nil)
	if err == nil {
		t.Fatal("loadPluginBytes() error = nil, want ID mismatch")
	}
	if !strings.Contains(err.Error(), `expected "demo", got "other"`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(loader.unloaded) != 1 || loader.unloaded[0] != "other" {
		t.Fatalf("unloaded = %#v, want [other]", loader.unloaded)
	}
}

func TestPluginLifecycleServiceReloadOrProbePlugin_ValidatesDisabledConfig(t *testing.T) {
	t.Parallel()

	svc := NewPluginLifecycleService(
		&testPluginStore{},
		nil,
		&adapter.Loader{},
		plugin.NewManager(),
		nil,
		nil,
		nil,
		nil,
		nil,
		PluginLifecycleOptions{},
	)
	svc.loader = &testLifecycleLoader{
		probeMeta: wasmrt.PluginMeta{
			ID:           "demo",
			ConfigSchema: json.RawMessage(`{"type":"object","properties":{"token":{"type":"string"}},"required":["token"]}`),
		},
	}

	_, err := svc.reloadOrProbePlugin(context.Background(), "demo", false, []byte("wasm"), json.RawMessage(`{}`), nil)
	if err == nil {
		t.Fatal("reloadOrProbePlugin() error = nil, want schema validation error")
	}
	if !strings.Contains(err.Error(), "token") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPluginLifecycleServiceReloadOrProbePlugin_EnabledSyncsManager(t *testing.T) {
	t.Parallel()

	manager := plugin.NewManager()
	manager.Register(&testLifecyclePlugin{id: "demo", name: "Old", version: "1.0.0"})

	svc := NewPluginLifecycleService(
		&testPluginStore{},
		nil,
		&adapter.Loader{},
		manager,
		nil,
		nil,
		nil,
		nil,
		nil,
		PluginLifecycleOptions{},
	)

	next := &testLifecyclePlugin{id: "demo", name: "Next", version: "2.0.0"}
	svc.loader = &testLifecycleLoader{
		plugins: map[string]lifecyclePlugin{
			"demo": next,
		},
	}

	meta, err := svc.reloadOrProbePlugin(context.Background(), "demo", true, []byte("wasm"), nil, map[string]struct{}{"old": {}})
	if err != nil {
		t.Fatalf("reloadOrProbePlugin() error = %v", err)
	}
	if meta.Version != "2.0.0" {
		t.Fatalf("meta.Version = %q, want %q", meta.Version, "2.0.0")
	}

	got, ok := manager.Get("demo")
	if !ok {
		t.Fatal("expected plugin manager to contain reloaded plugin")
	}
	if got.Version() != "2.0.0" {
		t.Fatalf("manager plugin version = %q, want %q", got.Version(), "2.0.0")
	}
}
