package model

type ChatGroup struct {
	ID    int64           `json:"id"`
	Name  string          `json:"name"`
	Chats []ChatReference `json:"chats,omitempty"`
}
