package plugin

import (
	"sync"
	"time"

	"SuperBotGo/internal/model"
)

// FocusTracker remembers the last plugin each user interacted with.
// The record expires after a configurable TTL of inactivity.
type FocusTracker struct {
	mu      sync.RWMutex
	entries map[model.GlobalUserID]focusEntry
	ttl     time.Duration
}

type focusEntry struct {
	pluginID  string
	timestamp time.Time
}

func NewFocusTracker(ttl time.Duration) *FocusTracker {
	return &FocusTracker{
		entries: make(map[model.GlobalUserID]focusEntry),
		ttl:     ttl,
	}
}

// Record stores a plugin interaction for the given user.
func (f *FocusTracker) Record(userID model.GlobalUserID, pluginID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries[userID] = focusEntry{pluginID: pluginID, timestamp: time.Now()}
}

// LastPlugin returns the most recently used plugin for the user,
// or "" if the focus session has expired or no interaction was recorded.
func (f *FocusTracker) LastPlugin(userID model.GlobalUserID) string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	entry, ok := f.entries[userID]
	if !ok || time.Since(entry.timestamp) > f.ttl {
		return ""
	}
	return entry.pluginID
}
