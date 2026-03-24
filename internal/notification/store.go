package notification

import (
	"context"
	"strings"

	"SuperBotGo/internal/model"
)

// PrefsRepository provides access to per-user notification preferences.
type PrefsRepository interface {
	GetPrefs(ctx context.Context, userID model.GlobalUserID) (*model.NotificationPrefs, error)
	SavePrefs(ctx context.Context, prefs *model.NotificationPrefs) error
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
