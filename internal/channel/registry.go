package channel

import (
	"context"
	"fmt"
	"sync"

	"SuperBotGo/internal/model"
)

// AdapterRegistry is a thread-safe store of ChannelAdapters keyed by ChannelType.
type AdapterRegistry struct {
	mu       sync.RWMutex
	adapters map[model.ChannelType]ChannelAdapter
}

// NewAdapterRegistry creates an empty AdapterRegistry.
func NewAdapterRegistry() *AdapterRegistry {
	return &AdapterRegistry{
		adapters: make(map[model.ChannelType]ChannelAdapter),
	}
}

// Register adds or replaces the adapter for the given channel type.
func (r *AdapterRegistry) Register(adapter ChannelAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[adapter.Type()] = adapter
}

// Get returns the adapter for the given channel type, or nil if none is registered.
func (r *AdapterRegistry) Get(channelType model.ChannelType) ChannelAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.adapters[channelType]
}

// MustGet returns the adapter for the given channel type or returns an error.
func (r *AdapterRegistry) MustGet(channelType model.ChannelType) (ChannelAdapter, error) {
	adapter := r.Get(channelType)
	if adapter == nil {
		return nil, fmt.Errorf("no adapter registered for channel type: %s", channelType)
	}
	return adapter, nil
}

// SendToChat sends a message to a chat via the appropriate adapter.
func (r *AdapterRegistry) SendToChat(ctx context.Context, channelType model.ChannelType, chatID string, msg model.Message) error {
	adapter, err := r.MustGet(channelType)
	if err != nil {
		return err
	}
	return adapter.SendToChat(ctx, chatID, msg)
}

// SendToUser sends a message to a user via the appropriate adapter.
func (r *AdapterRegistry) SendToUser(ctx context.Context, channelType model.ChannelType, platformUserID model.PlatformUserID, msg model.Message) error {
	adapter, err := r.MustGet(channelType)
	if err != nil {
		return err
	}
	return adapter.SendToUser(ctx, platformUserID, msg)
}
