package channel

import (
	"context"
	"fmt"
	"sync"

	"SuperBotGo/internal/model"
)

type AdapterRegistry struct {
	mu       sync.RWMutex
	adapters map[model.ChannelType]ChannelAdapter
}

func NewAdapterRegistry() *AdapterRegistry {
	return &AdapterRegistry{
		adapters: make(map[model.ChannelType]ChannelAdapter),
	}
}

func (r *AdapterRegistry) Register(adapter ChannelAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[adapter.Type()] = adapter
}

func (r *AdapterRegistry) Get(channelType model.ChannelType) ChannelAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.adapters[channelType]
}

func (r *AdapterRegistry) MustGet(channelType model.ChannelType) (ChannelAdapter, error) {
	adapter := r.Get(channelType)
	if adapter == nil {
		return nil, fmt.Errorf("no adapter registered for channel type: %s", channelType)
	}
	return adapter, nil
}

func (r *AdapterRegistry) SendToChat(ctx context.Context, channelType model.ChannelType, chatID string, msg model.Message) error {
	adapter, err := r.MustGet(channelType)
	if err != nil {
		return err
	}
	return adapter.SendToChat(ctx, chatID, msg)
}

func (r *AdapterRegistry) SendToUser(ctx context.Context, channelType model.ChannelType, platformUserID model.PlatformUserID, msg model.Message) error {
	adapter, err := r.MustGet(channelType)
	if err != nil {
		return err
	}
	return adapter.SendToUser(ctx, platformUserID, msg)
}
