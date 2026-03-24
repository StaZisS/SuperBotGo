package notification

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"SuperBotGo/internal/model"
)

// PrefsRepository provides access to per-user notification preferences.
type PrefsRepository interface {
	GetPrefs(ctx context.Context, userID model.GlobalUserID) (*model.NotificationPrefs, error)
	SavePrefs(ctx context.Context, prefs *model.NotificationPrefs) error
}

// PlaceholderPrefsRepo is an in-memory implementation for use when PostgreSQL is unavailable.
type PlaceholderPrefsRepo struct {
	mu    sync.RWMutex
	store map[model.GlobalUserID]*model.NotificationPrefs
}

func NewPlaceholderPrefsRepo() *PlaceholderPrefsRepo {
	return &PlaceholderPrefsRepo{
		store: make(map[model.GlobalUserID]*model.NotificationPrefs),
	}
}

func (r *PlaceholderPrefsRepo) GetPrefs(_ context.Context, userID model.GlobalUserID) (*model.NotificationPrefs, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.store[userID]
	if !ok {
		return nil, nil
	}
	cp := *p
	return &cp, nil
}

func (r *PlaceholderPrefsRepo) SavePrefs(_ context.Context, prefs *model.NotificationPrefs) error {
	if prefs == nil {
		return fmt.Errorf("notification prefs: nil prefs")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	stored := *prefs
	r.store[prefs.GlobalUserID] = &stored
	return nil
}

var _ PrefsRepository = (*PlaceholderPrefsRepo)(nil)

// MarshalChannelPriority encodes a slice of ChannelType to a comma-separated string.
func MarshalChannelPriority(channels []model.ChannelType) string {
	parts := make([]string, len(channels))
	for i, ch := range channels {
		parts[i] = string(ch)
	}
	return strings.Join(parts, ",")
}

// UnmarshalChannelPriority decodes a comma-separated string into a slice of ChannelType.
func UnmarshalChannelPriority(s string) []model.ChannelType {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]model.ChannelType, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, model.ChannelType(p))
		}
	}
	return result
}
