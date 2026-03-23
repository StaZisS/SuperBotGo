package hostapi

import (
	"fmt"
	"sync"
	"time"
)

const (
	kvMaxKeysPerPlugin  = 1000
	kvMaxValueSize      = 64 * 1024
	kvMaxTotalPerPlugin = 10 * 1024 * 1024
)

type kvEntry struct {
	value     string
	size      int
	expiresAt time.Time
	hasTTL    bool
}

func (e *kvEntry) isExpired(now time.Time) bool {
	return e.hasTTL && now.After(e.expiresAt)
}

type pluginKV struct {
	mu        sync.RWMutex
	entries   map[string]*kvEntry
	totalSize int
}

type KVStore struct {
	mu      sync.RWMutex
	plugins map[string]*pluginKV
}

func NewKVStore() *KVStore {
	return &KVStore{
		plugins: make(map[string]*pluginKV),
	}
}

func (s *KVStore) getOrCreatePluginKV(pluginID string) *pluginKV {
	s.mu.RLock()
	pk, ok := s.plugins[pluginID]
	s.mu.RUnlock()
	if ok {
		return pk
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if pk, ok = s.plugins[pluginID]; ok {
		return pk
	}
	pk = &pluginKV{
		entries: make(map[string]*kvEntry),
	}
	s.plugins[pluginID] = pk
	return pk
}

func (s *KVStore) Get(pluginID, key string) (string, bool, error) {
	pk := s.getOrCreatePluginKV(pluginID)

	pk.mu.RLock()
	entry, ok := pk.entries[key]
	pk.mu.RUnlock()

	if !ok {
		return "", false, nil
	}

	if entry.isExpired(time.Now()) {
		pk.mu.Lock()
		if e, exists := pk.entries[key]; exists && e.isExpired(time.Now()) {
			pk.totalSize -= e.size
			delete(pk.entries, key)
		}
		pk.mu.Unlock()
		return "", false, nil
	}

	return entry.value, true, nil
}

func (s *KVStore) Set(pluginID, key, value string, ttl time.Duration) error {
	if len(value) > kvMaxValueSize {
		return fmt.Errorf("value too large: %d bytes (max %d)", len(value), kvMaxValueSize)
	}

	pk := s.getOrCreatePluginKV(pluginID)
	entrySize := len(key) + len(value)

	pk.mu.Lock()
	defer pk.mu.Unlock()

	s.evictExpiredLocked(pk)

	oldSize := 0
	if old, exists := pk.entries[key]; exists {
		oldSize = old.size
	}

	if oldSize == 0 && len(pk.entries) >= kvMaxKeysPerPlugin {
		return fmt.Errorf("too many keys: max %d per plugin", kvMaxKeysPerPlugin)
	}

	newTotal := pk.totalSize - oldSize + entrySize
	if newTotal > kvMaxTotalPerPlugin {
		return fmt.Errorf("total storage exceeded: would use %d bytes (max %d)", newTotal, kvMaxTotalPerPlugin)
	}

	entry := &kvEntry{
		value: value,
		size:  entrySize,
	}
	if ttl > 0 {
		entry.hasTTL = true
		entry.expiresAt = time.Now().Add(ttl)
	}

	pk.entries[key] = entry
	pk.totalSize = newTotal
	return nil
}

func (s *KVStore) Delete(pluginID, key string) error {
	pk := s.getOrCreatePluginKV(pluginID)

	pk.mu.Lock()
	defer pk.mu.Unlock()

	if entry, exists := pk.entries[key]; exists {
		pk.totalSize -= entry.size
		delete(pk.entries, key)
	}
	return nil
}

func (s *KVStore) List(pluginID, prefix string) ([]string, error) {
	pk := s.getOrCreatePluginKV(pluginID)

	pk.mu.RLock()
	defer pk.mu.RUnlock()

	now := time.Now()
	keys := make([]string, 0, len(pk.entries))
	for k, entry := range pk.entries {
		if entry.isExpired(now) {
			continue
		}
		if prefix != "" && len(k) < len(prefix) {
			continue
		}
		if prefix != "" && k[:len(prefix)] != prefix {
			continue
		}
		keys = append(keys, k)
	}
	return keys, nil
}

func (s *KVStore) evictExpiredLocked(pk *pluginKV) {
	now := time.Now()
	for key, entry := range pk.entries {
		if entry.isExpired(now) {
			pk.totalSize -= entry.size
			delete(pk.entries, key)
		}
	}
}

func (s *KVStore) DropPlugin(pluginID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.plugins, pluginID)
}
