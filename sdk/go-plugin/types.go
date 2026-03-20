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
	Nodes       []nodeDef `json:"nodes,omitempty"`
}

type stepDef struct {
	Param      string      `json:"param"`
	Prompt     string      `json:"prompt"`
	Options    []optionDef `json:"options,omitempty"`
	Validation string      `json:"validation,omitempty"`
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

// ---------------------------------------------------------------------------
// Node tree types — used by both meta (serialization) and step_callback
// (callback registry reconstruction).
// ---------------------------------------------------------------------------

type nodeDef struct {
	Type             string               `json:"type"`                        // "step", "branch", "conditional_branch"
	Param            string               `json:"param,omitempty"`             // step
	Blocks           []blockDef           `json:"blocks,omitempty"`            // step
	Validation       string               `json:"validation,omitempty"`        // step: regex
	ValidateFn       string               `json:"validate_fn,omitempty"`       // step: WASM callback name
	VisibleWhen      *conditionDef        `json:"visible_when,omitempty"`      // step: declarative condition
	ConditionFn      string               `json:"condition_fn,omitempty"`      // step: WASM callback name
	Pagination       *paginationDef       `json:"pagination,omitempty"`        // step
	OnParam          string               `json:"on_param,omitempty"`          // branch
	Cases            map[string][]nodeDef `json:"cases,omitempty"`             // branch
	ConditionalCases []condCaseDef        `json:"conditional_cases,omitempty"` // conditional_branch
	Default          []nodeDef            `json:"default,omitempty"`           // branch / conditional_branch
}

type blockDef struct {
	Type      string      `json:"type"`                 // "text", "options", "dynamic_options", "link", "image"
	Text      string      `json:"text,omitempty"`       // text
	Style     string      `json:"style,omitempty"`      // text
	Prompt    string      `json:"prompt,omitempty"`     // options, dynamic_options
	Options   []optionDef `json:"options,omitempty"`    // options
	OptionsFn string      `json:"options_fn,omitempty"` // dynamic_options: WASM callback name
	URL       string      `json:"url,omitempty"`        // link, image
	Label     string      `json:"label,omitempty"`      // link
}

type conditionDef struct {
	Param string          `json:"param,omitempty"`
	Eq    *string         `json:"eq,omitempty"`
	Neq   *string         `json:"neq,omitempty"`
	Match string          `json:"match,omitempty"`
	Set   *bool           `json:"set,omitempty"`
	And   []*conditionDef `json:"and,omitempty"`
	Or    []*conditionDef `json:"or,omitempty"`
	Not   *conditionDef   `json:"not,omitempty"`
}

type paginationDef struct {
	Prompt   string `json:"prompt"`
	PageSize int    `json:"page_size"`
	Provider string `json:"provider"` // WASM callback name
}

type condCaseDef struct {
	Condition   *conditionDef `json:"condition,omitempty"`
	ConditionFn string        `json:"condition_fn,omitempty"` // WASM callback name
	Nodes       []nodeDef     `json:"nodes"`
}

// ---------------------------------------------------------------------------
// step_callback protocol
// ---------------------------------------------------------------------------

type stepCallbackRequest struct {
	Callback string            `json:"callback"`
	UserID   int64             `json:"user_id"`
	Locale   string            `json:"locale"`
	Params   map[string]string `json:"params"`
	Page     int               `json:"page"`
	Input    string            `json:"input"`
}

type stepCallbackResponse struct {
	Options []optionDef `json:"options,omitempty"`
	HasMore bool        `json:"has_more,omitempty"`
	Result  *bool       `json:"result,omitempty"`
	Error   string      `json:"error,omitempty"`
}
