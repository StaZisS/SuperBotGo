package model

type Project struct {
	ID          int64         `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Bindings    []ChatBinding `json:"bindings,omitempty"`
}
