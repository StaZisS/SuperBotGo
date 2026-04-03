package plugin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/model"
)

type SenderUserService interface {
	GetUser(ctx context.Context, id model.GlobalUserID) (*model.GlobalUser, error)
}

type SenderAPI struct {
	adapters    *channel.AdapterRegistry
	userService SenderUserService
}

func NewSenderAPI(adapters *channel.AdapterRegistry, userService SenderUserService) *SenderAPI {
	return &SenderAPI{
		adapters:    adapters,
		userService: userService,
	}
}

func (s *SenderAPI) Reply(ctx context.Context, m *model.MessengerTriggerData, msg model.Message) error {
	return s.adapters.SendToChat(ctx, m.ChannelType, m.ChatID, msg)
}

func (s *SenderAPI) resolveUser(ctx context.Context, userID model.GlobalUserID) (*model.GlobalUser, error) {
	user, err := s.userService.GetUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("sender: get user %d: %w", userID, err)
	}
	if user == nil {
		return nil, fmt.Errorf("sender: user %d not found", userID)
	}
	return user, nil
}

func (s *SenderAPI) SendToUser(ctx context.Context, userID model.GlobalUserID, msg model.Message) error {
	user, err := s.resolveUser(ctx, userID)
	if err != nil {
		return err
	}

	platformID := user.PlatformUserID()
	if platformID == "" {
		return fmt.Errorf("sender: user %d has no platform account on channel %s", userID, user.PrimaryChannel)
	}

	return s.adapters.SendToUser(ctx, user.PrimaryChannel, platformID, msg)
}

func (s *SenderAPI) SendToAllChannels(ctx context.Context, userID model.GlobalUserID, msg model.Message) error {
	user, err := s.resolveUser(ctx, userID)
	if err != nil {
		return err
	}

	var errs []error
	for _, account := range user.Accounts {
		if err := s.adapters.SendToUser(ctx, account.ChannelType, account.ChannelUserID, msg); err != nil {
			slog.Error("sender: partial failure sending to user",
				slog.Int64("user_id", int64(userID)),
				slog.String("channel", string(account.ChannelType)),
				slog.Any("error", err))
			errs = append(errs, fmt.Errorf("%s: %w", account.ChannelType, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("sender: send to user %d failed on %d/%d channels: %w",
			userID, len(errs), len(user.Accounts), errors.Join(errs...))
	}
	return nil
}

func (s *SenderAPI) ReplyToChat(ctx context.Context, channelType model.ChannelType, chatID string, msg model.Message) error {
	return s.adapters.SendToChat(ctx, channelType, chatID, msg)
}
