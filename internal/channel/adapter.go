package channel

import (
	"context"

	"SuperBotGo/internal/model"
)

// ChannelAdapter is the interface each messaging platform must implement.
type ChannelAdapter interface {
	// Type returns the channel type this adapter handles (e.g. TELEGRAM, DISCORD).
	Type() model.ChannelType
	// SendToUser sends a message to a specific user by their platform-specific ID.
	SendToUser(ctx context.Context, platformUserID model.PlatformUserID, msg model.Message) error
	// SendToChat sends a message to a chat/channel by its platform-specific ID.
	SendToChat(ctx context.Context, chatID string, msg model.Message) error
}

// UpdateHandler receives incoming updates from platform adapters.
type UpdateHandler interface {
	// OnUpdate is called when a user sends input (text or callback) via a platform.
	OnUpdate(ctx context.Context, channelType model.ChannelType, platformUserID model.PlatformUserID, input model.UserInput, chatID string) error
}
