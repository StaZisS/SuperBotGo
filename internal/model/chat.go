package model

type ChatKind string

const (
	ChatKindGroup   ChatKind = "GROUP"
	ChatKindPrivate ChatKind = "PRIVATE"
	ChatKindChannel ChatKind = "CHANNEL"
)

type ChatReference struct {
	ID             int64          `json:"id"`
	ChannelType    ChannelType    `json:"channel_type"`
	PlatformChatID string         `json:"platform_chat_id"`
	ChatKind       ChatKind       `json:"chat_kind"`
	Title          string         `json:"title"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type Project struct {
	ID          int64         `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Bindings    []ChatBinding `json:"bindings,omitempty"`
}

type ChatBinding struct {
	ID        int64 `json:"id"`
	ProjectID int64 `json:"project_id"`
	ChatRefID int64 `json:"chat_ref_id"`
}

type ChatGroup struct {
	ID    int64           `json:"id"`
	Name  string          `json:"name"`
	Chats []ChatReference `json:"chats,omitempty"`
}
