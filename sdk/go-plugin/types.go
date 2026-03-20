package wasmplugin

import "encoding/json"

// ---------------------------------------------------------------------------
// Internal JSON types — mirrors of the host protocol.
// All types are unexported; the public API uses Plugin / Command / Step, etc.
// ---------------------------------------------------------------------------

type pluginMeta struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Version      string          `json:"version"`
	SDKVersion   int             `json:"sdk_version"`
	Commands     []commandDef    `json:"commands,omitempty"`
	Permissions  []permissionDef `json:"permissions,omitempty"`
	ConfigSchema json.RawMessage `json:"config_schema,omitempty"`
}

type commandDef struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	MinRole     string    `json:"min_role,omitempty"`
	Steps       []stepDef `json:"steps,omitempty"`
}

type stepDef struct {
	Param      string            `json:"param"`
	Prompt     string            `json:"prompt"`
	Options    []optionDef       `json:"options,omitempty"`
	Validation string            `json:"validation,omitempty"`
	Vars       map[string]string `json:"vars,omitempty"`
}

type optionDef struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type permissionDef struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type commandRequest struct {
	UserID      int64             `json:"user_id"`
	ChannelType string            `json:"channel_type"`
	ChatID      string            `json:"chat_id"`
	CommandName string            `json:"command_name"`
	Params      map[string]string `json:"params"`
	Locale      string            `json:"locale"`
}

type logEntry struct {
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

type messageEntry struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

type responseJSON struct {
	Status   string         `json:"status,omitempty"`
	Error    string         `json:"error,omitempty"`
	Reply    string         `json:"reply,omitempty"`
	Logs     []logEntry     `json:"logs,omitempty"`
	Messages []messageEntry `json:"messages,omitempty"`
}
