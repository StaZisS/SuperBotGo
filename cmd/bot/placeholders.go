package main

import (
	"context"
	"log/slog"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/chat"
	"SuperBotGo/internal/model"
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
