package notification

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/model"
)

// UserService resolves a global user by ID.
type UserService interface {
	GetUser(ctx context.Context, id model.GlobalUserID) (*model.GlobalUser, error)
}

// StudentResolver resolves a university hierarchy scope to a list of global user IDs.
type StudentResolver interface {
	ResolveStudentUsers(ctx context.Context, scope string, targetID int64) ([]model.GlobalUserID, error)
}

// NotifyAPI is the high-level notification service that respects user preferences
// (channel priority, mute mentions, work hours) when delivering messages.
type NotifyAPI struct {
	adapters *channel.AdapterRegistry
	users    UserService
	prefs    PrefsRepository
	students StudentResolver
}

func NewNotifyAPI(
	adapters *channel.AdapterRegistry,
	users UserService,
	prefs PrefsRepository,
	students StudentResolver,
) *NotifyAPI {
	return &NotifyAPI{
		adapters: adapters,
		users:    users,
		prefs:    prefs,
		students: students,
	}
}

// NotifyUser sends a notification to a user with priority-aware delivery.
func (n *NotifyAPI) NotifyUser(ctx context.Context, userID model.GlobalUserID, msg model.Message, priority model.NotifyPriority) error {
	user, err := n.users.GetUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("notify: get user %d: %w", userID, err)
	}
	if user == nil {
		return fmt.Errorf("notify: user %d not found", userID)
	}

	prefs, err := n.prefs.GetPrefs(ctx, userID)
	if err != nil {
		return fmt.Errorf("notify: get prefs for user %d: %w", userID, err)
	}
	if prefs == nil {
		prefs = defaultPrefs(userID, user.PrimaryChannel)
	}

	opts := n.buildSendOptions(prefs, priority)

	if priority == model.PriorityCritical {
		var sendErrs []error
		for _, acc := range user.Accounts {
			msgCopy := n.maybeInjectMention(msg, acc.ChannelUserID, prefs, priority)
			if err := n.adapters.SendToUserWithOpts(ctx, acc.ChannelType, acc.ChannelUserID, msgCopy, opts); err != nil {
				slog.Error("notify: partial failure sending to user",
					slog.Int64("user_id", int64(userID)),
					slog.String("channel", string(acc.ChannelType)),
					slog.Any("error", err))
				sendErrs = append(sendErrs, fmt.Errorf("%s: %w", acc.ChannelType, err))
			}
		}
		if len(sendErrs) > 0 {
			return fmt.Errorf("notify: user %d critical send failed on %d/%d channels: %w",
				userID, len(sendErrs), len(user.Accounts), errors.Join(sendErrs...))
		}
		return nil
	}

	targetChannel, platformID := n.resolveChannel(user, prefs)

	msg = n.maybeInjectMention(msg, platformID, prefs, priority)
	return n.adapters.SendToUserWithOpts(ctx, targetChannel, platformID, msg, opts)
}

// NotifyChat sends a notification to a specific chat with priority-aware delivery.
func (n *NotifyAPI) NotifyChat(ctx context.Context, channelType model.ChannelType, chatID string, msg model.Message, priority model.NotifyPriority) error {
	opts := model.SendOptions{
		Silent: priority == model.PriorityLow,
	}
	return n.adapters.SendToChatWithOpts(ctx, channelType, chatID, msg, opts)
}

// NotifyStudents sends a priority-aware notification to all students within
// the given university hierarchy scope.
// It continues sending to remaining users even if some fail.
func (n *NotifyAPI) NotifyStudents(ctx context.Context, scope string, targetID int64, msg model.Message, priority model.NotifyPriority) error {
	userIDs, err := n.students.ResolveStudentUsers(ctx, scope, targetID)
	if err != nil {
		return fmt.Errorf("notify: resolve students for %s/%d: %w", scope, targetID, err)
	}

	var sendErrs []error
	for _, uid := range userIDs {
		if err := n.NotifyUser(ctx, uid, msg, priority); err != nil {
			slog.Error("notify: partial failure sending to student",
				slog.String("scope", scope),
				slog.Int64("target_id", targetID),
				slog.Int64("user_id", int64(uid)),
				slog.Any("error", err))
			sendErrs = append(sendErrs, fmt.Errorf("user %d: %w", uid, err))
		}
	}
	if len(sendErrs) > 0 {
		return fmt.Errorf("notify: students %s/%d broadcast failed on %d/%d users: %w",
			scope, targetID, len(sendErrs), len(userIDs), errors.Join(sendErrs...))
	}
	return nil
}

// buildSendOptions determines Silent and StripMentions based on prefs and priority.
func (n *NotifyAPI) buildSendOptions(prefs *model.NotificationPrefs, priority model.NotifyPriority) model.SendOptions {
	var opts model.SendOptions

	if priority == model.PriorityLow && !isWithinWorkHours(prefs) {
		opts.Silent = true
	}

	if prefs.MuteMentions && priority < model.PriorityCritical {
		opts.StripMentions = true
	}

	return opts
}

// resolveChannel picks the target channel based on user preferences.
func (n *NotifyAPI) resolveChannel(user *model.GlobalUser, prefs *model.NotificationPrefs) (model.ChannelType, model.PlatformUserID) {
	accountMap := make(map[model.ChannelType]model.PlatformUserID, len(user.Accounts))
	for _, acc := range user.Accounts {
		accountMap[acc.ChannelType] = acc.ChannelUserID
	}

	for _, ch := range prefs.ChannelPriority {
		if pid, ok := accountMap[ch]; ok {
			return ch, pid
		}
	}

	if pid, ok := accountMap[user.PrimaryChannel]; ok {
		return user.PrimaryChannel, pid
	}

	if len(user.Accounts) > 0 {
		acc := user.Accounts[0]
		return acc.ChannelType, acc.ChannelUserID
	}

	return user.PrimaryChannel, ""
}

func (n *NotifyAPI) maybeInjectMention(msg model.Message, platformUserID model.PlatformUserID, prefs *model.NotificationPrefs, priority model.NotifyPriority) model.Message {
	if priority < model.PriorityHigh {
		return msg
	}
	if prefs.MuteMentions && priority < model.PriorityCritical {
		return msg
	}

	for _, block := range msg.Blocks {
		if m, ok := block.(model.MentionBlock); ok && m.UserID == string(platformUserID) {
			return msg
		}
	}

	blocks := make([]model.ContentBlock, 0, len(msg.Blocks)+1)
	blocks = append(blocks, model.MentionBlock{UserID: string(platformUserID)})
	blocks = append(blocks, msg.Blocks...)
	return model.Message{Blocks: blocks}
}

func isWithinWorkHours(prefs *model.NotificationPrefs) bool {
	if prefs.WorkHoursStart == nil || prefs.WorkHoursEnd == nil {
		return true
	}

	tz := prefs.Timezone
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return true
	}

	hour := time.Now().In(loc).Hour()
	start, end := *prefs.WorkHoursStart, *prefs.WorkHoursEnd

	if start <= end {
		return hour >= start && hour < end
	}
	return hour >= start || hour < end
}

func defaultPrefs(userID model.GlobalUserID, primaryChannel model.ChannelType) *model.NotificationPrefs {
	return &model.NotificationPrefs{
		GlobalUserID:    userID,
		ChannelPriority: []model.ChannelType{primaryChannel},
		MuteMentions:    false,
		Timezone:        "UTC",
	}
}
