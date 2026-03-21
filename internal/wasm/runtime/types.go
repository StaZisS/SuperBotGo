package runtime

import "encoding/json"

type PluginIDKey struct{}

const MaxSupportedSDKVersion = 1

type PluginMeta struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Version      string          `json:"version"`
	SDKVersion   int             `json:"sdk_version"`
	Commands     []CommandDef    `json:"commands,omitempty"`
	Triggers     []TriggerDef    `json:"triggers,omitempty"`
	Permissions  []PermissionDef `json:"permissions,omitempty"`
	ConfigSchema json.RawMessage `json:"config_schema,omitempty"`
}

// TriggerDef declares a trigger the plugin wants to handle.
type TriggerDef struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"` // "http", "cron", "event"
	Description string   `json:"description,omitempty"`
	Path        string   `json:"path,omitempty"`     // HTTP
	Methods     []string `json:"methods,omitempty"`  // HTTP
	Schedule    string   `json:"schedule,omitempty"` // Cron
	Topic       string   `json:"topic,omitempty"`    // Event
}

type CommandDef struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	MinRole     string    `json:"min_role,omitempty"`
	Steps       []StepDef `json:"steps,omitempty"`
	Nodes       []NodeDef `json:"nodes,omitempty"`
}

type StepDef struct {
	Param      string      `json:"param"`
	Prompt     string      `json:"prompt"`
	Options    []OptionDef `json:"options,omitempty"`
	Validation string      `json:"validation,omitempty"`
}

type OptionDef struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type PermissionDef struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type NodeDef struct {
	Type             string               `json:"type"`
	Param            string               `json:"param,omitempty"`
	Blocks           []BlockDef           `json:"blocks,omitempty"`
	Validation       string               `json:"validation,omitempty"`
	ValidateFn       string               `json:"validate_fn,omitempty"`
	VisibleWhen      *ConditionDef        `json:"visible_when,omitempty"`
	ConditionFn      string               `json:"condition_fn,omitempty"`
	Pagination       *PaginationNodeDef   `json:"pagination,omitempty"`
	OnParam          string               `json:"on_param,omitempty"`
	Cases            map[string][]NodeDef `json:"cases,omitempty"`
	ConditionalCases []CondCaseDef        `json:"conditional_cases,omitempty"`
	Default          []NodeDef            `json:"default,omitempty"`
}

type BlockDef struct {
	Type      string      `json:"type"`
	Text      string      `json:"text,omitempty"`
	Style     string      `json:"style,omitempty"`
	Prompt    string      `json:"prompt,omitempty"`
	Options   []OptionDef `json:"options,omitempty"`
	OptionsFn string      `json:"options_fn,omitempty"`
	URL       string      `json:"url,omitempty"`
	Label     string      `json:"label,omitempty"`
}

type ConditionDef struct {
	Param string          `json:"param,omitempty"`
	Eq    *string         `json:"eq,omitempty"`
	Neq   *string         `json:"neq,omitempty"`
	Match string          `json:"match,omitempty"`
	Set   *bool           `json:"set,omitempty"`
	And   []*ConditionDef `json:"and,omitempty"`
	Or    []*ConditionDef `json:"or,omitempty"`
	Not   *ConditionDef   `json:"not,omitempty"`
}

type PaginationNodeDef struct {
	Prompt   string `json:"prompt"`
	PageSize int    `json:"page_size"`
	Provider string `json:"provider"`
}

type CondCaseDef struct {
	Condition   *ConditionDef `json:"condition,omitempty"`
	ConditionFn string        `json:"condition_fn,omitempty"`
	Nodes       []NodeDef     `json:"nodes"`
}

type StepCallbackRequest struct {
	Callback string            `json:"callback"`
	UserID   int64             `json:"user_id"`
	Locale   string            `json:"locale"`
	Params   map[string]string `json:"params"`
	Page     int               `json:"page"`
	Input    string            `json:"input"`
}

type StepCallbackResponse struct {
	Options []OptionDef `json:"options,omitempty"`
	HasMore bool        `json:"has_more,omitempty"`
	Result  *bool       `json:"result,omitempty"`
	Error   string      `json:"error,omitempty"`
}
