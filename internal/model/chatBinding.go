package model

type ChatBinding struct {
	ID        int64 `json:"id"`
	ProjectID int64 `json:"project_id"`
	ChatRefID int64 `json:"chat_ref_id"`
}
