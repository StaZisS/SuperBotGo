package notification

import (
	"context"
	"strings"
	"time"

	"SuperBotGo/internal/model"
)

// PrefsRepository provides access to per-user notification preferences.
type PrefsRepository interface {
	GetPrefs(ctx context.Context, userID model.GlobalUserID) (*model.NotificationPrefs, error)
	SavePrefs(ctx context.Context, prefs *model.NotificationPrefs) error
}

// ScheduledMessage is a user notification delayed until the recipient's work hours.
type ScheduledMessage struct {
	ID        int64
	UserID    model.GlobalUserID
	Msg       model.Message
	Priority  model.NotifyPriority
	SendAt    time.Time
	CreatedAt time.Time
	Attempts  int
}

// ScheduledMessageStore persists delayed notifications and leases due work to workers.
type ScheduledMessageStore interface {
	Enqueue(ctx context.Context, msg ScheduledMessage) error
	ClaimDue(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]ScheduledMessage, error)
	Complete(ctx context.Context, id int64) error
	Reschedule(ctx context.Context, id int64, sendAt time.Time, reason error) error
}

// MarshalChannelPriority encodes a slice of ChannelType to a comma-separated string.
func MarshalChannelPriority(channels []model.ChannelType) string {
	parts := make([]string, len(channels))
	for i, ch := range channels {
		parts[i] = string(ch)
	}
	return strings.Join(parts, ",")
}

// UnmarshalChannelPriority decodes a comma-separated string into a slice of ChannelType.
func UnmarshalChannelPriority(s string) []model.ChannelType {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]model.ChannelType, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, model.ChannelType(p))
		}
	}
	return result
}
