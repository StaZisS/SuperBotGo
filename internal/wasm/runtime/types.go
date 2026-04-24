package runtime

import "encoding/json"

type PluginIDKey struct{}
type HTTPAuthDataKey struct{}

const MaxSupportedSDKVersion = 3

const ActionMigrate = "migrate"
const ActionReconfigure = "reconfigure"
const ActionHandleRPC = "handle_rpc"

type PluginMeta struct {
	ID                  string           `json:"id"`
	Name                string           `json:"name"`
	Version             string           `json:"version"`
	SDKVersion          int              `json:"sdk_version"`
	SupportsReconfigure bool             `json:"supports_reconfigure,omitempty"`
	RPCMethods          []RPCMethodDef   `json:"rpc_methods,omitempty"`
	Triggers            []TriggerDef     `json:"triggers,omitempty"`
	Requirements        []RequirementDef `json:"requirements,omitempty"`
	ConfigSchema        json.RawMessage  `json:"config_schema,omitempty"`
	Dependencies        []DependencyDef  `json:"dependencies,omitempty"`
	Migrations          []MigrationDef   `json:"migrations,omitempty"`
}

type ReconfigureRequest struct {
	PreviousConfig json.RawMessage `json:"previous_config,omitempty"`
	Config         json.RawMessage `json:"config,omitempty"`
}

type RPCMethodDef struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type RPCRequest struct {
	Caller string `json:"caller,omitempty"`
	Method string `json:"method"`
	Params []byte `json:"params,omitempty"`
}

type RPCResponse struct {
	Status string `json:"status"`
	Result []byte `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

type DependencyDef struct {
	PluginID          string `json:"plugin"`
	VersionConstraint string `json:"version"`
}

// MigrationDef describes a single SQL migration declared by a plugin.
type MigrationDef struct {
	Version     int    `json:"version"`
	Description string `json:"description"`
	Up          string `json:"up"`
	Down        string `json:"down,omitempty"`
}

type TriggerDef struct {
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Description string    `json:"description,omitempty"`
	Path        string    `json:"path,omitempty"`
	Methods     []string  `json:"methods,omitempty"`
	Schedule    string    `json:"schedule,omitempty"`
	Topic       string    `json:"topic,omitempty"`
	Nodes       []NodeDef `json:"nodes,omitempty"`
}

type OptionDef struct {
	Label  string            `json:"label"`
	Labels map[string]string `json:"labels,omitempty"`
	Value  string            `json:"value"`
}

type RequirementDef struct {
	Type        string          `json:"type"`
	Description string          `json:"description,omitempty"`
	Name        string          `json:"name,omitempty"`
	Target      string          `json:"target,omitempty"`
	Config      json.RawMessage `json:"config,omitempty"`
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
	Type      string            `json:"type"`
	Text      string            `json:"text,omitempty"`
	Texts     map[string]string `json:"texts,omitempty"`
	Style     string            `json:"style,omitempty"`
	Prompt    string            `json:"prompt,omitempty"`
	Prompts   map[string]string `json:"prompts,omitempty"`
	Options   []OptionDef       `json:"options,omitempty"`
	OptionsFn string            `json:"options_fn,omitempty"`
	URL       string            `json:"url,omitempty"`
	Label     string            `json:"label,omitempty"`
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
	Prompt   string            `json:"prompt"`
	Prompts  map[string]string `json:"prompts,omitempty"`
	PageSize int               `json:"page_size"`
	Provider string            `json:"provider"`
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
