package main

import (
	"context"
	"log/slog"
	"sync"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/chat"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/state/storage"
)

type registryChatJoinHandler struct {
	registry chat.Registry
	logger   *slog.Logger
}

func newChatJoinHandler(registry chat.Registry, logger *slog.Logger) *registryChatJoinHandler {
	return &registryChatJoinHandler{registry: registry, logger: logger}
}

func (h *registryChatJoinHandler) OnChatJoin(ctx context.Context, channelType model.ChannelType, platformChatID string, chatKind model.ChatKind, title string) error {
	ref, err := h.registry.FindOrCreateChat(ctx, channelType, platformChatID, chatKind, title)
	if err != nil {
		return err
	}
	h.logger.Info("chat registered",
		slog.Int64("id", ref.ID),
		slog.String("channel_type", string(ref.ChannelType)),
		slog.String("platform_chat_id", ref.PlatformChatID),
		slog.String("chat_kind", string(ref.ChatKind)),
		slog.String("title", ref.Title))
	return nil
}

func (h *registryChatJoinHandler) OnChatLeave(ctx context.Context, channelType model.ChannelType, platformChatID string) error {
	if err := h.registry.UnregisterChatByPlatformID(ctx, channelType, platformChatID); err != nil {
		return err
	}
	h.logger.Info("chat unregistered",
		slog.String("channel_type", string(channelType)),
		slog.String("platform_chat_id", platformChatID))
	return nil
}

var _ channel.ChatJoinHandler = (*registryChatJoinHandler)(nil)

type inMemoryDialogStorage struct {
	mu    sync.RWMutex
	store map[model.GlobalUserID]*model.DialogState
}

func (s *inMemoryDialogStorage) Save(_ context.Context, userID model.GlobalUserID, ds model.DialogState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store == nil {
		s.store = make(map[model.GlobalUserID]*model.DialogState)
	}
	copy := ds
	s.store[userID] = &copy
	return nil
}

func (s *inMemoryDialogStorage) Load(_ context.Context, userID model.GlobalUserID) (*model.DialogState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return nil, nil
	}
	ds, ok := s.store[userID]
	if !ok {
		return nil, nil
	}
	copy := *ds
	return &copy, nil
}

func (s *inMemoryDialogStorage) Delete(_ context.Context, userID model.GlobalUserID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store != nil {
		delete(s.store, userID)
	}
	return nil
}

var _ storage.DialogStorage = (*inMemoryDialogStorage)(nil)
