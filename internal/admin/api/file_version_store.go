package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type FileVersionStore struct {
	mu     sync.RWMutex
	path   string
	data   []VersionRecord
	nextID int64
}

func NewFileVersionStore(modulesDir string) (*FileVersionStore, error) {
	if err := os.MkdirAll(modulesDir, 0o755); err != nil {
		return nil, fmt.Errorf("create modules dir: %w", err)
	}

	path := filepath.Join(modulesDir, "versions.json")
	s := &FileVersionStore{
		path:   path,
		nextID: 1,
	}

	raw, err := os.ReadFile(path)
	if err == nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, &s.data); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		for _, r := range s.data {
			if r.ID >= s.nextID {
				s.nextID = r.ID + 1
			}
		}
	}

	return s, nil
}

func (s *FileVersionStore) SaveVersion(_ context.Context, rec VersionRecord) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec.ID = s.nextID
	s.nextID++
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now()
	}
	s.data = append(s.data, rec)
	return rec.ID, s.flush()
}

func (s *FileVersionStore) ListVersions(_ context.Context, pluginID string) ([]VersionRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []VersionRecord
	// Return in reverse order (newest first)
	for i := len(s.data) - 1; i >= 0; i-- {
		if s.data[i].PluginID == pluginID {
			result = append(result, s.data[i])
		}
	}
	return result, nil
}

func (s *FileVersionStore) GetVersion(_ context.Context, id int64) (VersionRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, r := range s.data {
		if r.ID == id {
			return r, nil
		}
	}
	return VersionRecord{}, fmt.Errorf("version %d not found", id)
}

func (s *FileVersionStore) DeleteVersion(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, r := range s.data {
		if r.ID == id {
			s.data = append(s.data[:i], s.data[i+1:]...)
			return s.flush()
		}
	}
	return fmt.Errorf("version %d not found", id)
}

func (s *FileVersionStore) DeleteVersionsByPlugin(_ context.Context, pluginID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := s.data[:0]
	for _, r := range s.data {
		if r.PluginID != pluginID {
			filtered = append(filtered, r)
		}
	}
	s.data = filtered
	return s.flush()
}

func (s *FileVersionStore) flush() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal versions: %w", err)
	}
	if err := os.WriteFile(s.path, raw, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", s.path, err)
	}
	return nil
}

var _ VersionStore = (*FileVersionStore)(nil)
