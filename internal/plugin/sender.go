package plugin

import (
	"context"
	"fmt"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/model"
)

// SenderUserService is the user service interface needed by SenderAPI.
type SenderUserService interface {
	GetUser(ctx context.Context, id model.GlobalUserID) (*model.GlobalUser, error)
}

// SenderChatRegistry is the chat registry interface needed by SenderAPI.
type SenderChatRegistry interface {
	FindChatsByProject(ctx context.Context, projectID int64) ([]model.ChatReference, error)
}

// SenderAPI provides plugins with the ability to send messages to users and chats.
type SenderAPI struct {
	adapters    *channel.AdapterRegistry
	userService SenderUserService
	chatReg     SenderChatRegistry
}

// NewSenderAPI creates a new SenderAPI.
func NewSenderAPI(adapters *channel.AdapterRegistry, userService SenderUserService, chatReg SenderChatRegistry) *SenderAPI {
	return &SenderAPI{
		adapters:    adapters,
		userService: userService,
		chatReg:     chatReg,
	}
}

// Reply sends a message back to the chat from which the command was issued.
func (s *SenderAPI) Reply(ctx context.Context, req model.CommandRequest, msg model.Message) error {
	return s.adapters.SendToChat(ctx, req.ChannelType, req.ChatID, msg)
}

// SendToUser sends a message to a specific user via their primary channel.
func (s *SenderAPI) SendToUser(ctx context.Context, userID model.GlobalUserID, msg model.Message) error {
	user, err := s.userService.GetUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("sender: get user %d: %w", userID, err)
	}
	if user == nil {
		return fmt.Errorf("sender: user %d not found", userID)
	}

	platformID := user.PlatformUserID()
	if platformID == "" {
		return fmt.Errorf("sender: user %d has no platform account on channel %s", userID, user.PrimaryChannel)
	}

	return s.adapters.SendToUser(ctx, user.PrimaryChannel, platformID, msg)
}

// SendToAllChannels sends a message to a user on all their linked channels.
func (s *SenderAPI) SendToAllChannels(ctx context.Context, userID model.GlobalUserID, msg model.Message) error {
	user, err := s.userService.GetUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("sender: get user %d: %w", userID, err)
	}
	if user == nil {
		return fmt.Errorf("sender: user %d not found", userID)
	}

	for _, account := range user.Accounts {
		if err := s.adapters.SendToUser(ctx, account.ChannelType, account.ChannelUserID, msg); err != nil {
			return fmt.Errorf("sender: send to user %d on %s: %w", userID, account.ChannelType, err)
		}
	}
	return nil
}

// SendToProject sends a message to all chats bound to a project.
func (s *SenderAPI) SendToProject(ctx context.Context, projectID int64, msg model.Message) error {
	chats, err := s.chatReg.FindChatsByProject(ctx, projectID)
	if err != nil {
		return fmt.Errorf("sender: find chats for project %d: %w", projectID, err)
	}

	for _, chat := range chats {
		adapter := s.adapters.Get(chat.ChannelType)
		if adapter == nil {
			continue
		}
		if err := adapter.SendToChat(ctx, chat.PlatformChatID, msg); err != nil {
			return fmt.Errorf("sender: send to project %d chat %s: %w", projectID, chat.PlatformChatID, err)
		}
	}
	return nil
}

// ReplyToChat sends a message to a specific chat on a specific channel.
func (s *SenderAPI) ReplyToChat(ctx context.Context, channelType model.ChannelType, chatID string, msg model.Message) error {
	return s.adapters.SendToChat(ctx, channelType, chatID, msg)
}
