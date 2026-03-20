package api

import (
	"context"
	"fmt"
	"sync"
)

type MemoryPluginStore struct {
	mu      sync.RWMutex
	records map[string]PluginRecord
}

func NewMemoryPluginStore() *MemoryPluginStore {
	return &MemoryPluginStore{
		records: make(map[string]PluginRecord),
	}
}

func (s *MemoryPluginStore) SavePlugin(_ context.Context, record PluginRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[record.ID] = record
	return nil
}

func (s *MemoryPluginStore) GetPlugin(_ context.Context, id string) (PluginRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.records[id]
	if !ok {
		return PluginRecord{}, fmt.Errorf("plugin %q not found", id)
	}
	return rec, nil
}

func (s *MemoryPluginStore) ListPlugins(_ context.Context) ([]PluginRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]PluginRecord, 0, len(s.records))
	for _, rec := range s.records {
		result = append(result, rec)
	}
	return result, nil
}

func (s *MemoryPluginStore) DeletePlugin(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.records[id]; !ok {
		return fmt.Errorf("plugin %q not found", id)
	}
	delete(s.records, id)
	return nil
}
