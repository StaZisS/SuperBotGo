package model

type ChatReference struct {
	ID             int64          `json:"id"`
	ChannelType    ChannelType    `json:"channel_type"`
	PlatformChatID string         `json:"platform_chat_id"`
	ChatKind       ChatKind       `json:"chat_kind"`
	Title          string         `json:"title"`
	Locale         string         `json:"locale,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}
