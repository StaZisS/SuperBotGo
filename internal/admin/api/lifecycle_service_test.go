package api

import (
	"context"
	"encoding/json"
	"testing"

	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/wasm/adapter"
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
