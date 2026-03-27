package model

type OptionMap map[string]string

func (m OptionMap) Get(key string) string {
	if m == nil {
		return ""
	}
	return m[key]
}

type CommandRequest struct {
	UserID      GlobalUserID `json:"user_id"`
	ChannelType ChannelType  `json:"channel_type"`
	ChatID      string       `json:"chat_id"`
	CommandName string       `json:"command_name"`
	Params      OptionMap    `json:"params,omitempty"`
	Locale      string       `json:"locale"`
}

type StepResult int

const (
	StepContinue StepResult = iota
	StepComplete
	StepInvalid
)

type DialogState struct {
	UserID      GlobalUserID   `json:"user_id"`
	ChatID      string         `json:"chat_id"`
	CommandName string         `json:"command_name"`
	Params      OptionMap      `json:"params,omitempty"`
	PageState   map[string]int `json:"page_state,omitempty"`
	CreatedAt   int64          `json:"created_at"`
}
