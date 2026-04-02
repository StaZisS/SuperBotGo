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
	PluginID    string       `json:"plugin_id,omitempty"`
	CommandName string       `json:"command_name"`
	Params      OptionMap    `json:"params,omitempty"`
	Locale      string       `json:"locale"`
}

// CommandCandidate describes one plugin's claim on a command alias.
// Used during disambiguation when multiple plugins register the same short name.
type CommandCandidate struct {
	PluginID    string
	CommandName string // short alias
	FQName      string // plugin_id.command_name
	Description string
}

type DialogState struct {
	UserID      GlobalUserID   `json:"user_id"`
	ChatID      string         `json:"chat_id"`
	PluginID    string         `json:"plugin_id,omitempty"`
	CommandName string         `json:"command_name"`
	Params      OptionMap      `json:"params,omitempty"`
	PageState   map[string]int `json:"page_state,omitempty"`
	CreatedAt   int64          `json:"created_at"`
}
