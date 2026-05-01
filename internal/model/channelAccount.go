package model

import "time"

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
	PlatformID    int64          `json:"platform_id"`
	ChannelType   ChannelType    `json:"channel_type"`
	ChannelUserID PlatformUserID `json:"channel_user_id"`
	GlobalUserID  GlobalUserID   `json:"global_user_id"`
	Username      string         `json:"username,omitempty"`
	FirstName     string         `json:"first_name,omitempty"`
	LastName      string         `json:"last_name,omitempty"`
	AvatarURL     string         `json:"avatar_url,omitempty"`
	IsLinked      bool           `json:"is_linked"`
	LinkedAt      *time.Time     `json:"linked_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}