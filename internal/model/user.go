package model

// GlobalUserID is the internal unique identifier for a user across all platforms.
type GlobalUserID int64

// PlatformUserID is the user identifier as known by a specific messaging platform.
type PlatformUserID string

// ChannelType identifies which messaging platform a user or chat belongs to.
type ChannelType string

const (
	ChannelTelegram ChannelType = "TELEGRAM"
	ChannelDiscord  ChannelType = "DISCORD"
)

// GlobalUser represents a user that may be linked to multiple platform accounts.
type GlobalUser struct {
	ID             GlobalUserID     `json:"id"`
	TsuAccountsID  *int64           `json:"tsu_accounts_id,omitempty"`
	PrimaryChannel ChannelType      `json:"primary_channel"`
	ProfileData    map[string]any   `json:"profile_data,omitempty"`
	Locale         string           `json:"locale"`
	Role           string           `json:"role"`
	Accounts       []ChannelAccount `json:"accounts,omitempty"`
}

// PlatformUserID returns the PlatformUserID for the user's primary channel,
// or empty string if no matching account is found.
func (u *GlobalUser) PlatformUserID() PlatformUserID {
	for _, acc := range u.Accounts {
		if acc.ChannelType == u.PrimaryChannel {
			return acc.ChannelUserID
		}
	}
	return ""
}

// ChannelAccount links a platform-specific user identity to a GlobalUser.
type ChannelAccount struct {
	ID            int64          `json:"id"`
	ChannelType   ChannelType    `json:"channel_type"`
	ChannelUserID PlatformUserID `json:"channel_user_id"`
	GlobalUserID  GlobalUserID   `json:"global_user_id"`
}
