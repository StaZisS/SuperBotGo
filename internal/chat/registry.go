package chat

import (
	"context"

	"SuperBotGo/internal/model"
)

type Registry interface {
	FindOrCreateChat(ctx context.Context, channelType model.ChannelType, chatID string, kind model.ChatKind, title string) (*model.ChatReference, error)
	FindChat(ctx context.Context, channelType model.ChannelType, platformChatID string) (*model.ChatReference, error)
	FindChatsByProject(ctx context.Context, projectID int64) ([]model.ChatReference, error)
	RegisterChat(ctx context.Context, ref model.ChatReference) (*model.ChatReference, error)
	UnregisterChat(ctx context.Context, chatRefID int64) error
	UnregisterChatByPlatformID(ctx context.Context, channelType model.ChannelType, platformChatID string) error
	UpdateChatLocale(ctx context.Context, chatRefID int64, locale string) error
}
