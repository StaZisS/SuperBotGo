package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type FilePluginStore struct {
	mu   sync.RWMutex
	path string
	data map[string]PluginRecord
}

func NewFilePluginStore(modulesDir string) (*FilePluginStore, error) {
	if err := os.MkdirAll(modulesDir, 0o755); err != nil {
		return nil, fmt.Errorf("create modules dir: %w", err)
	}

	path := filepath.Join(modulesDir, "plugins.json")
	s := &FilePluginStore{
		path: path,
		data: make(map[string]PluginRecord),
	}

	raw, err := os.ReadFile(path)
	if err == nil && len(raw) > 0 {
		var records []PluginRecord
		if err := json.Unmarshal(raw, &records); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		for _, r := range records {
			s.data[r.ID] = r
		}
	}

	return s, nil
}

func (s *FilePluginStore) SavePlugin(_ context.Context, record PluginRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[record.ID] = record
	return s.flush()
}

func (s *FilePluginStore) GetPlugin(_ context.Context, id string) (PluginRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.data[id]
	if !ok {
		return PluginRecord{}, fmt.Errorf("plugin %q not found", id)
	}
	return rec, nil
}

func (s *FilePluginStore) ListPlugins(_ context.Context) ([]PluginRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]PluginRecord, 0, len(s.data))
	for _, rec := range s.data {
		result = append(result, rec)
	}
	return result, nil
}

func (s *FilePluginStore) DeletePlugin(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[id]; !ok {
		return fmt.Errorf("plugin %q not found", id)
	}
	delete(s.data, id)
	return s.flush()
}

func (s *FilePluginStore) flush() error {
	records := make([]PluginRecord, 0, len(s.data))
	for _, r := range s.data {
		records = append(records, r)
	}
	raw, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal plugins: %w", err)
	}
	if err := os.WriteFile(s.path, raw, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", s.path, err)
	}
	return nil
}

var _ PluginStore = (*FilePluginStore)(nil)
