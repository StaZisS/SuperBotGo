package wasmplugin

import "encoding/json"

// ---------------------------------------------------------------------------
// Internal JSON types — mirrors of the host protocol.
// All types are unexported; the public API uses Plugin / Trigger / Node, etc.
// ---------------------------------------------------------------------------

type pluginMeta struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Version      string           `json:"version"`
	SDKVersion   int              `json:"sdk_version"`
	Triggers     []triggerDef     `json:"triggers,omitempty"`
	Requirements []requirementDef `json:"requirements,omitempty"`
	ConfigSchema json.RawMessage  `json:"config_schema,omitempty"`
	Migrations   []migrationDef   `json:"migrations,omitempty"`
}

type triggerDef struct {
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Description string    `json:"description,omitempty"`
	Path        string    `json:"path,omitempty"`
	Methods     []string  `json:"methods,omitempty"`
	Schedule    string    `json:"schedule,omitempty"`
	Topic       string    `json:"topic,omitempty"`
	Nodes       []nodeDef `json:"nodes,omitempty"`
}

type optionDef struct {
	Label  string            `json:"label"`
	Labels map[string]string `json:"labels,omitempty"`
	Value  string            `json:"value"`
}

type requirementDef struct {
	Type        string          `json:"type"`
	Description string          `json:"description,omitempty"`
	Target      string          `json:"target,omitempty"`
	Config      json.RawMessage `json:"config,omitempty"`
}

type migrationDef struct {
	Version     int    `json:"version"`
	Description string `json:"description"`
	Up          string `json:"up"`
	Down        string `json:"down,omitempty"`
}

type logEntry struct {
	Level string `json:"level"`
	Msg   string `json:"msg"`
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
	Type      string            `json:"type"`                 // "text", "options", "dynamic_options", "link", "image"
	Texts     map[string]string `json:"texts,omitempty"`      // text: localized
	Style     string            `json:"style,omitempty"`      // text
	Prompts   map[string]string `json:"prompts,omitempty"`    // options, dynamic_options: localized
	Options   []optionDef       `json:"options,omitempty"`    // options
	OptionsFn string            `json:"options_fn,omitempty"` // dynamic_options: WASM callback name
	URL       string            `json:"url,omitempty"`        // link, image
	Label     string            `json:"label,omitempty"`      // link
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
	Prompts  map[string]string `json:"prompts,omitempty"` // localized
	PageSize int               `json:"page_size"`
	Provider string            `json:"provider"` // WASM callback name
}

type condCaseDef struct {
	Condition   *conditionDef `json:"condition,omitempty"`
	ConditionFn string        `json:"condition_fn,omitempty"` // WASM callback name
	Nodes       []nodeDef     `json:"nodes"`
}

// ---------------------------------------------------------------------------
// migrate protocol
// ---------------------------------------------------------------------------

type migrateRequest struct {
	OldVersion string `json:"old_version"`
	NewVersion string `json:"new_version"`
}

type migrateResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
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

// ---------------------------------------------------------------------------
// handle_event protocol
// ---------------------------------------------------------------------------

type eventRequest struct {
	ID          string          `json:"id"`
	TriggerType string          `json:"trigger_type"`
	TriggerName string          `json:"trigger_name"`
	PluginID    string          `json:"plugin_id"`
	Timestamp   int64           `json:"timestamp"`
	Data        json.RawMessage `json:"data"`
}

type eventResponseJSON struct {
	Status     string            `json:"status,omitempty"`
	Error      string            `json:"error,omitempty"`
	Reply      string            `json:"reply,omitempty"`
	ReplyTexts map[string]string `json:"reply_texts,omitempty"`
	Data       json.RawMessage   `json:"data,omitempty"`
	Logs       []logEntry        `json:"logs,omitempty"`
}

type messengerTriggerData struct {
	UserID      int64             `json:"user_id"`
	ChannelType string            `json:"channel_type"`
	ChatID      string            `json:"chat_id"`
	CommandName string            `json:"command_name"`
	Params      map[string]string `json:"params"`
	Locale      string            `json:"locale"`
}

type httpTriggerData struct {
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Query      map[string]string `json:"query,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	RemoteAddr string            `json:"remote_addr,omitempty"`
}

type httpResponseData struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body"`
}

type cronTriggerData struct {
	ScheduleName string `json:"schedule_name"`
	FireTime     int64  `json:"fire_time"`
}

type eventTriggerData struct {
	Topic   string          `json:"topic"`
	Payload json.RawMessage `json:"payload"`
	Source  string          `json:"source"`
}
