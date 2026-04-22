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
const ProtocolVersion = 3

// SQLMigration describes a single SQL schema migration step.
// Migrations are declared in the Plugin struct and serialized as part of
// the meta response. The host applies them via goose before calling configure.
type SQLMigration struct {
	Version     int    // Sequential version number (1, 2, 3, ...)
	Description string // Short description (e.g. "create_schedule_entries")
	Up          string // SQL to apply
	Down        string // SQL to rollback (optional)
}

type RPCHandler func(ctx *RPCContext) ([]byte, error)

type RPCMethod struct {
	Name        string
	Description string
	Handler     RPCHandler
}

// Plugin defines a WASM plugin. Fill this struct and pass it to [Run].
type Plugin struct {
	ID           string
	Name         string
	Version      string
	RPCMethods   []RPCMethod
	Triggers     []Trigger
	Requirements []Requirement

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

	// OnReconfigure is called when PLUGIN_ACTION=reconfigure.
	// previousConfig contains the currently persisted config, config contains
	// the new config to apply.
	OnReconfigure func(previousConfig, config []byte) error

	// OnEvent is the fallback event handler for triggers that don't have
	// their own Handler. If a Trigger has a Handler, it takes priority.
	OnEvent func(ctx *EventContext) error

	// Migrate is called when the host detects a version change during plugin
	// reload. The handler receives a MigrateContext with the old and new
	// version strings plus access to the KV store for data transformation.
	// If nil, migration is a silent no-op (success).
	Migrate func(ctx *MigrateContext) error

	// Migrations declares SQL schema migrations that the host will run
	// via goose before calling OnConfigure. Each migration has a version
	// number, a description, and Up/Down SQL statements.
	// This is separate from the Migrate callback (which handles KV data
	// migration on version changes).
	Migrations []SQLMigration
}

// TriggerType identifies the kind of trigger.
type TriggerType = string

const (
	TriggerHTTP      TriggerType = "http"
	TriggerCron      TriggerType = "cron"
	TriggerEvent     TriggerType = "event"
	TriggerMessenger TriggerType = "messenger"
)

// Trigger declares a trigger source this plugin responds to.
// For messenger triggers this also describes the interactive command flow.
type Trigger struct {
	Name        string
	Type        TriggerType
	Description string

	// Messenger-specific: interactive command flow (node tree with
	// branching, pagination, dynamic options, conditions).
	Nodes []Node

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

// Option is a single choice in a step with predefined values.
type Option struct {
	Label  string            // single-locale label (backward compatible)
	Labels map[string]string // localized labels keyed by locale (e.g. "en", "ru"); host resolves
	Value  string
}

// Requirement declares a host resource the plugin needs.
// All declared requirements are mandatory — the plugin will not load
// unless every requirement is fulfilled.
type Requirement struct {
	Type        string // "database", "http", "kv", "notify", "events", "plugin"
	Description string
	Name        string       // logical name (e.g. database alias); defaults to "default"
	Target      string       // for "plugin" type: target plugin ID
	Config      ConfigSchema // config the admin must fill
}

// RequirementBuilder provides a fluent API for constructing requirements.
type RequirementBuilder struct {
	r Requirement
}

// Name sets a logical name for the requirement (e.g. a database alias).
// For Database requirements this controls which connection the plugin
// opens via sql.Open("superbot", name). Defaults to "default".
func (b *RequirementBuilder) Name(n string) *RequirementBuilder {
	b.r.Name = n
	return b
}

func (b *RequirementBuilder) WithConfig(cs ConfigSchema) *RequirementBuilder {
	b.r.Config = cs
	return b
}

func (b *RequirementBuilder) Build() Requirement { return b.r }

// Database declares a requirement for SQL database access.
// The admin provides connection strings via the structured "databases"
// section in the plugin config — no need to add DSN fields manually.
//
// Single database (name defaults to "default"):
//
//	wasmplugin.Database("Store schedule entries").Build()
//
// Multiple databases:
//
//	wasmplugin.Database("Main storage").Build(),
//	wasmplugin.Database("Read replica").Name("analytics").Build(),
//
// In plugin code, open the connection by name:
//
//	db, _ := sql.Open("superbot", "")            // "default"
//	db, _ := sql.Open("superbot", "analytics")   // named
func Database(desc string) *RequirementBuilder {
	return &RequirementBuilder{r: Requirement{
		Type:        "database",
		Description: desc,
		Name:        "default",
	}}
}

// HTTP declares a requirement for outbound HTTP requests.
func HTTP(desc string) *RequirementBuilder {
	return &RequirementBuilder{r: Requirement{Type: "http", Description: desc}}
}

// HTTPPolicyConfig returns a standard config schema for host-enforced outbound
// HTTP restrictions under the reserved requirements.http.<name> namespace.
func HTTPPolicyConfig() ConfigSchema {
	return ConfigFields(
		StringArray("allowed_hosts", "Allowed outbound hostnames").Required(),
		StringArray("allowed_methods", "Allowed HTTP methods"),
		Integer("max_request_body_bytes", "Maximum request body size in bytes").Min(0),
		Integer("max_response_body_bytes", "Maximum response body size in bytes").Min(0),
	)
}

// KV declares a requirement for key-value store access.
func KV(desc string) *RequirementBuilder {
	return &RequirementBuilder{r: Requirement{Type: "kv", Description: desc}}
}

// NotifyReq declares a requirement for sending notifications
// (NotifyUser, NotifyChat, NotifyStudents).
func NotifyReq(desc string) *RequirementBuilder {
	return &RequirementBuilder{r: Requirement{Type: "notify", Description: desc}}
}

// EventsReq declares a requirement for publishing events.
func EventsReq(desc string) *RequirementBuilder {
	return &RequirementBuilder{r: Requirement{Type: "events", Description: desc}}
}

// PluginDep declares a requirement for calling another plugin.
func PluginDep(target, desc string) *RequirementBuilder {
	return &RequirementBuilder{r: Requirement{Type: "plugin", Description: desc, Target: target}}
}

// File declares a requirement for file store access (read, write, metadata).
func File(desc string) *RequirementBuilder {
	return &RequirementBuilder{r: Requirement{Type: "file", Description: desc}}
}

// configStore holds the parsed plugin configuration (set during configure action,
// passed to handlers via PLUGIN_CONFIG env var).
var configStore map[string]interface{}
