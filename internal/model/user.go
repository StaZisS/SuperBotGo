package model

type GlobalUserID int64

type PlatformUserID string

type ChannelType string

const (
	ChannelTelegram ChannelType = "TELEGRAM"
	ChannelDiscord  ChannelType = "DISCORD"
)

type GlobalUser struct {
	ID             GlobalUserID     `json:"id"`
	TsuAccountsID  *int64           `json:"tsu_accounts_id,omitempty"`
	PrimaryChannel ChannelType      `json:"primary_channel"`
	ProfileData    map[string]any   `json:"profile_data,omitempty"`
	Locale         string           `json:"locale"`
	Role           string           `json:"role"`
	Accounts       []ChannelAccount `json:"accounts,omitempty"`
}

func (u *GlobalUser) PlatformUserID() PlatformUserID {
	for _, acc := range u.Accounts {
		if acc.ChannelType == u.PrimaryChannel {
			return acc.ChannelUserID
		}
	}
	return ""
}

type ChannelAccount struct {
	ID            int64          `json:"id"`
	ChannelType   ChannelType    `json:"channel_type"`
	ChannelUserID PlatformUserID `json:"channel_user_id"`
	GlobalUserID  GlobalUserID   `json:"global_user_id"`
}
