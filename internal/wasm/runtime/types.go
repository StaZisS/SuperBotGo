package runtime

import "encoding/json"

// PluginIDKey is the context key used to pass the plugin ID to host functions.
// Host functions retrieve it via ctx.Value(PluginIDKey{}).(string).
type PluginIDKey struct{}

// MaxSupportedSDKVersion is the highest SDK protocol version the host supports.
// Plugins built with a higher version will be rejected at load time.
const MaxSupportedSDKVersion = 1

// PluginMeta holds metadata returned by the plugin's "meta" export.
type PluginMeta struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Version      string          `json:"version"`
	SDKVersion   int             `json:"sdk_version"`
	Commands     []CommandDef    `json:"commands,omitempty"`
	Permissions  []PermissionDef `json:"permissions,omitempty"`
	ConfigSchema json.RawMessage `json:"config_schema,omitempty"`
}

// CommandDef describes a single command provided by a plugin.
type CommandDef struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	MinRole     string    `json:"min_role,omitempty"`
	Steps       []StepDef `json:"steps,omitempty"`
	Nodes       []NodeDef `json:"nodes,omitempty"`
}

// StepDef describes a single dialog step for collecting a parameter.
type StepDef struct {
	Param      string      `json:"param"`
	Prompt     string      `json:"prompt"`
	Options    []OptionDef `json:"options,omitempty"`
	Validation string      `json:"validation,omitempty"`
}

// OptionDef describes a selectable option in a step.
type OptionDef struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// PermissionDef describes a permission a plugin may request.
type PermissionDef struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// ---------------------------------------------------------------------------
// Extended node tree (used when CommandDef.Nodes is non-empty).
// ---------------------------------------------------------------------------

// NodeDef describes a node in the command flow tree.
// The Type field acts as a discriminator: "step", "branch", "conditional_branch".
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

// BlockDef describes a single content block inside a step prompt.
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

// ConditionDef is a declarative condition evaluated on the host without a WASM call.
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

// PaginationNodeDef configures paginated option selection for a step.
type PaginationNodeDef struct {
	Prompt   string `json:"prompt"`
	PageSize int    `json:"page_size"`
	Provider string `json:"provider"` // WASM callback name
}

// CondCaseDef is a single case in a conditional branch.
type CondCaseDef struct {
	Condition   *ConditionDef `json:"condition,omitempty"`
	ConditionFn string        `json:"condition_fn,omitempty"`
	Nodes       []NodeDef     `json:"nodes"`
}

// ---------------------------------------------------------------------------
// step_callback protocol
// ---------------------------------------------------------------------------

// StepCallbackRequest is sent to the plugin when the host needs a runtime
// callback (validation, dynamic options, pagination, condition evaluation).
type StepCallbackRequest struct {
	Callback string            `json:"callback"`
	UserID   int64             `json:"user_id"`
	Locale   string            `json:"locale"`
	Params   map[string]string `json:"params"`
	Page     int               `json:"page"`
	Input    string            `json:"input"`
}

// StepCallbackResponse is the plugin's reply to a step_callback request.
type StepCallbackResponse struct {
	Options []OptionDef `json:"options,omitempty"`
	HasMore bool        `json:"has_more,omitempty"`
	Result  *bool       `json:"result,omitempty"`
	Error   string      `json:"error,omitempty"`
}
