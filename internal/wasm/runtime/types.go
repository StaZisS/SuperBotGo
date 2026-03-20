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
}

// StepDef describes a single dialog step for collecting a parameter.
type StepDef struct {
	Param      string            `json:"param"`
	Prompt     string            `json:"prompt"`
	Options    []OptionDef       `json:"options,omitempty"`
	Validation string            `json:"validation,omitempty"`
	Vars       map[string]string `json:"vars,omitempty"`
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
