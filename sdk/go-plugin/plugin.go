// Package wasmplugin provides an SDK for writing WASM plugins for SuperBotGo.
//
// A plugin author fills in a [Plugin] struct and calls [Run] from main().
// The SDK handles the one-shot protocol (meta / configure / handle_command),
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
	Handler     func(ctx *CommandContext) error
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

// CommandContext holds everything a command handler needs.
type CommandContext struct {
	UserID      int64
	ChannelType string
	ChatID      string
	CommandName string
	Params      map[string]string
	Locale      string

	reply    string // set by Reply, read by the runtime
	config   map[string]interface{}
	logs     []logEntry     // collected by Log/LogError
	messages []messageEntry // collected by SendMessage
}

// Reply sets the text that will be returned to the user via stdout.
// Only the last call to Reply is used.
func (ctx *CommandContext) Reply(text string) {
	ctx.reply = text
}

// Log records an informational log message.
// All collected logs are sent to the host after the handler completes.
func (ctx *CommandContext) Log(msg string) {
	ctx.logs = append(ctx.logs, logEntry{Level: "info", Msg: msg})
}

// LogError records an error log message.
// All collected logs are sent to the host after the handler completes.
func (ctx *CommandContext) LogError(msg string) {
	ctx.logs = append(ctx.logs, logEntry{Level: "error", Msg: msg})
}

// SendMessage queues a message to be sent to the given chat.
// Messages are delivered by the host after the handler completes.
func (ctx *CommandContext) SendMessage(chatID string, text string) {
	ctx.messages = append(ctx.messages, messageEntry{ChatID: chatID, Text: text})
}

// Config returns a config value by key, or the fallback if not set.
func (ctx *CommandContext) Config(key string, fallback string) string {
	if ctx.config == nil {
		return fallback
	}
	v, ok := ctx.config[key]
	if !ok {
		return fallback
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fallback
}

// configStore holds the parsed plugin configuration (set during configure action,
// passed to handlers via PLUGIN_CONFIG env var).
var configStore map[string]interface{}
