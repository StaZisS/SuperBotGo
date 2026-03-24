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

func (r *AdapterRegistry) mustGet(channelType model.ChannelType) (ChannelAdapter, error) {
	adapter := r.Get(channelType)
	if adapter == nil {
		return nil, fmt.Errorf("no adapter registered for channel type: %s", channelType)
	}
	return adapter, nil
}

// IsRegistered reports whether an adapter for the given channel type exists.
func (r *AdapterRegistry) IsRegistered(channelType model.ChannelType) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.adapters[channelType] != nil
}

// SendToChat dispatches a message to the appropriate adapter with retry on transient errors.
func (r *AdapterRegistry) SendToChat(ctx context.Context, channelType model.ChannelType, chatID string, msg model.Message) error {
	adapter, err := r.mustGet(channelType)
	if err != nil {
		return err
	}
	return withRetry(ctx, func() error {
		return adapter.SendToChat(ctx, chatID, msg)
	})
}

// SendToUser dispatches a message to the appropriate adapter with retry on transient errors.
func (r *AdapterRegistry) SendToUser(ctx context.Context, channelType model.ChannelType, platformUserID model.PlatformUserID, msg model.Message) error {
	adapter, err := r.mustGet(channelType)
	if err != nil {
		return err
	}
	return withRetry(ctx, func() error {
		return adapter.SendToUser(ctx, platformUserID, msg)
	})
}

// SendToChatWithOpts dispatches a message applying SendOptions (silent mode, mention stripping)
// with retry on transient errors.
func (r *AdapterRegistry) SendToChatWithOpts(ctx context.Context, channelType model.ChannelType, chatID string, msg model.Message, opts model.SendOptions) error {
	adapter, err := r.mustGet(channelType)
	if err != nil {
		return err
	}
	if opts.StripMentions {
		msg = model.StripMentionBlocks(msg)
	}
	return withRetry(ctx, func() error {
		if opts.Silent {
			if ss, ok := adapter.(SilentSender); ok {
				return ss.SendToChatSilent(ctx, chatID, msg, true)
			}
		}
		return adapter.SendToChat(ctx, chatID, msg)
	})
}

// SendToUserWithOpts dispatches a message applying SendOptions (silent mode, mention stripping)
// with retry on transient errors.
func (r *AdapterRegistry) SendToUserWithOpts(ctx context.Context, channelType model.ChannelType, platformUserID model.PlatformUserID, msg model.Message, opts model.SendOptions) error {
	adapter, err := r.mustGet(channelType)
	if err != nil {
		return err
	}
	if opts.StripMentions {
		msg = model.StripMentionBlocks(msg)
	}
	return withRetry(ctx, func() error {
		if opts.Silent {
			if ss, ok := adapter.(SilentSender); ok {
				return ss.SendToUserSilent(ctx, platformUserID, msg, true)
			}
		}
		return adapter.SendToUser(ctx, platformUserID, msg)
	})
}
