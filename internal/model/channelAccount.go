package model

type ChannelType string

const (
	ChannelTelegram   ChannelType = "TELEGRAM"
	ChannelDiscord    ChannelType = "DISCORD"
	ChannelVK         ChannelType = "VK"
	ChannelMattermost ChannelType = "MATTERMOST"
	ChannelWeb        ChannelType = "WEB"
)

type ChannelAccount struct {
	ID            int64          `json:"id"`
	ChannelType   ChannelType    `json:"channel_type"`
	ChannelUserID PlatformUserID `json:"channel_user_id"`
	GlobalUserID  GlobalUserID   `json:"global_user_id"`
	Username      string         `json:"username,omitempty"`
}
