package channel

import (
	"context"

	"SuperBotGo/internal/model"
)

type ChannelAdapter interface {
	Type() model.ChannelType
	SendToUser(ctx context.Context, platformUserID model.PlatformUserID, msg model.Message) error
	SendToChat(ctx context.Context, chatID string, msg model.Message) error
}

type ChatJoinHandler interface {
	OnChatJoin(ctx context.Context, channelType model.ChannelType, platformChatID string, chatKind model.ChatKind, title string) error
	OnChatLeave(ctx context.Context, channelType model.ChannelType, platformChatID string) error
}
