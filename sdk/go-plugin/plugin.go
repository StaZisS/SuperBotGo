// Package wasmplugin provides an SDK for writing WASM plugins for SuperBotGo.
//
// A plugin author fills in a [Plugin] struct and calls [Run] from main().
// The SDK handles the one-shot protocol (meta / configure / handle_event),
// JSON serialisation, the bump allocator, and host-function imports.
package wasmplugin

// ProtocolVersion is the SDK protocol version.
// The host checks this to ensure compatibility with the loaded plugin.
// Bump this when the protocol between host and plugin changes
// (e.g. new env vars, new response fields, changed JSON schema).
const ProtocolVersion = 1

// Plugin defines a WASM plugin. Fill this struct and pass it to [Run].
type Plugin struct {
	ID          string
	Name        string
	Version     string
	Commands    []Command
	Triggers    []Trigger
	Permissions []Permission

	// Config defines the plugin's configuration schema using the builder API.
	// The admin UI will generate a form from this.
	//
	//   Config: wasmplugin.ConfigFields(
	//       wasmplugin.String("greeting", "Welcome message"),
	//       wasmplugin.Integer("timeout", "Timeout in seconds").Default(30).Min(1).Max(300),
	//       wasmplugin.Bool("verbose", "Enable verbose output"),
	//       wasmplugin.Enum("theme", "Color theme", "light", "dark"),
	//   ),
	Config ConfigSchema

	// OnConfigure is called when PLUGIN_ACTION=configure.
	// config is the raw JSON read from stdin (may be empty).
	// Use [GetConfig] inside handlers to access the stored config.
	// Return nil to indicate success.
	OnConfigure func(config []byte) error

	// OnEvent is the fallback event handler for triggers that don't have
	// their own Handler. If a Trigger has a Handler, it takes priority.
	OnEvent func(ctx *EventContext) error

	// Migrate is called when the host detects a version change during plugin
	// reload. The handler receives a MigrateContext with the old and new
	// version strings plus access to the KV store for data transformation.
	// If nil, migration is a silent no-op (success).
	Migrate func(ctx *MigrateContext) error
}

// TriggerType identifies the kind of trigger.
type TriggerType = string

const (
	TriggerHTTP  TriggerType = "http"
	TriggerCron  TriggerType = "cron"
	TriggerEvent TriggerType = "event"
)

// Trigger declares a trigger source this plugin responds to.
type Trigger struct {
	Name        string
	Type        TriggerType
	Description string

	// HTTP-specific.
	Path    string   // e.g. "/webhook"
	Methods []string // e.g. ["POST"]

	// Cron-specific.
	Schedule string // cron expression, e.g. "0 */5 * * *"

	// Event-specific.
	Topic string // event topic to subscribe to

	// Handler is called when this trigger fires.
	// If nil, the plugin's OnEvent is used instead.
	Handler func(ctx *EventContext) error
}

// Command describes a single slash-command the plugin provides.
//
// Use either Steps (simple, flat list) or Nodes (full node tree with branching,
// pagination, dynamic options, conditions). If Nodes is set, Steps is ignored.
type Command struct {
	Name        string
	Description string
	MinRole     string // optional: minimum role required
	Steps       []Step // simple mode (flat list of steps)
	Nodes       []Node // advanced mode (node tree)
	Handler     func(ctx *EventContext) error
}

// Step describes one parameter-collection step in a multi-step command.
// This is the legacy flat format. For advanced features (branching, pagination,
// dynamic options, conditions) use [Command.Nodes] with [NewStep] instead.
type Step struct {
	Param      string   // parameter key
	Prompt     string   // text shown to the user
	Options    []Option // if non-empty, render as buttons/choices
	Validation string   // optional regex the value must match
}

// Option is a single choice in a step with predefined values.
type Option struct {
	Label string
	Value string
}

// Permission declares a host permission the plugin requires.
type Permission struct {
	Key         string
	Description string
	Required    bool
}

// configStore holds the parsed plugin configuration (set during configure action,
// passed to handlers via PLUGIN_CONFIG env var).
var configStore map[string]interface{}
