package model

// ChatKind categorises the type of chat on a messaging platform.
type ChatKind string

const (
	ChatKindGroup   ChatKind = "GROUP"
	ChatKindPrivate ChatKind = "PRIVATE"
	ChatKindChannel ChatKind = "CHANNEL"
)

// ChatReference represents a chat on a specific messaging platform.
type ChatReference struct {
	ID             int64          `json:"id"`
	ChannelType    ChannelType    `json:"channel_type"`
	PlatformChatID string         `json:"platform_chat_id"`
	ChatKind       ChatKind       `json:"chat_kind"`
	Title          string         `json:"title"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// Project groups related chat bindings under a named entity.
type Project struct {
	ID          int64         `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Bindings    []ChatBinding `json:"bindings,omitempty"`
}

// ChatBinding associates a Project with a ChatReference.
type ChatBinding struct {
	ID        int64 `json:"id"`
	ProjectID int64 `json:"project_id"`
	ChatRefID int64 `json:"chat_ref_id"`
}

// ChatGroup is a logical grouping of ChatReferences.
type ChatGroup struct {
	ID    int64           `json:"id"`
	Name  string          `json:"name"`
	Chats []ChatReference `json:"chats,omitempty"`
}
