package model

// OptionMap holds named parameters for a command invocation.
type OptionMap map[string]string

// Get returns the value for the given key, or an empty string if absent.
func (m OptionMap) Get(key string) string {
	if m == nil {
		return ""
	}
	return m[key]
}

// GetOr returns the value for the given key, or the provided default if absent.
func (m OptionMap) GetOr(key, defaultVal string) string {
	if m == nil {
		return defaultVal
	}
	v, ok := m[key]
	if !ok {
		return defaultVal
	}
	return v
}

// CommandRequest carries everything needed to start executing a command.
type CommandRequest struct {
	UserID      GlobalUserID `json:"user_id"`
	ChannelType ChannelType  `json:"channel_type"`
	ChatID      string       `json:"chat_id"`
	CommandName string       `json:"command_name"`
	Params      OptionMap    `json:"params,omitempty"`
	Locale      string       `json:"locale"`
}

// StepResult indicates the outcome of a single dialog step.
type StepResult int

const (
	// StepContinue means the dialog should advance to the next step.
	StepContinue StepResult = iota
	// StepComplete means the dialog has finished successfully.
	StepComplete
	// StepInvalid means the user's input was invalid; re-prompt.
	StepInvalid
)

// DialogState tracks the progress of a multi-step command dialog.
type DialogState struct {
	UserID      GlobalUserID   `json:"user_id"`
	CommandName string         `json:"command_name"`
	Params      OptionMap      `json:"params,omitempty"`
	PageState   map[string]int `json:"page_state,omitempty"`
	CreatedAt   int64          `json:"created_at"`
}
