package chat

import (
	"context"
	"sync"
	"sync/atomic"

	"SuperBotGo/internal/model"
)

// Registry manages chat references and their project associations.
type Registry interface {
	FindOrCreateChat(ctx context.Context, channelType model.ChannelType, chatID string, kind model.ChatKind, title string) (*model.ChatReference, error)
	FindChat(ctx context.Context, channelType model.ChannelType, platformChatID string) (*model.ChatReference, error)
	FindChatsByProject(ctx context.Context, projectID int64) ([]model.ChatReference, error)
	RegisterChat(ctx context.Context, ref model.ChatReference) (*model.ChatReference, error)
	UnregisterChat(ctx context.Context, chatRefID int64) error
}

// PlaceholderRegistry is an in-memory placeholder for Registry.
// It will be replaced with a database-backed implementation later.
type PlaceholderRegistry struct {
	mu    sync.RWMutex
	chats []*model.ChatReference
	seq   atomic.Int64
}

// NewPlaceholderRegistry creates a new in-memory chat registry.
func NewPlaceholderRegistry() *PlaceholderRegistry {
	return &PlaceholderRegistry{}
}

func (r *PlaceholderRegistry) FindOrCreateChat(ctx context.Context, channelType model.ChannelType, chatID string, kind model.ChatKind, title string) (*model.ChatReference, error) {
	existing, _ := r.FindChat(ctx, channelType, chatID)
	if existing != nil {
		return existing, nil
	}
	return r.RegisterChat(ctx, model.ChatReference{
		ChannelType:    channelType,
		PlatformChatID: chatID,
		ChatKind:       kind,
		Title:          title,
	})
}

func (r *PlaceholderRegistry) FindChat(_ context.Context, channelType model.ChannelType, platformChatID string) (*model.ChatReference, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, ch := range r.chats {
		if ch.ChannelType == channelType && ch.PlatformChatID == platformChatID {
			copy := *ch
			return &copy, nil
		}
	}
	return nil, nil
}

func (r *PlaceholderRegistry) FindChatsByProject(_ context.Context, _ int64) ([]model.ChatReference, error) {

	return nil, nil
}

func (r *PlaceholderRegistry) RegisterChat(_ context.Context, ref model.ChatReference) (*model.ChatReference, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ref.ID = r.seq.Add(1)
	stored := ref
	r.chats = append(r.chats, &stored)
	result := stored
	return &result, nil
}

func (r *PlaceholderRegistry) UnregisterChat(_ context.Context, chatRefID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, ch := range r.chats {
		if ch.ID == chatRefID {
			r.chats = append(r.chats[:i], r.chats[i+1:]...)
			return nil
		}
	}
	return nil
}

var _ Registry = (*PlaceholderRegistry)(nil)
