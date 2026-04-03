package wasmplugin

import "encoding/json"

// EventContext is the unified context for all event handlers (commands, HTTP, cron, events).
type EventContext struct {
	PluginID    string
	TriggerType string
	TriggerName string
	Timestamp   int64

	// Messenger is non-nil for messenger command events.
	Messenger *MessengerData
	// HTTP is non-nil for HTTP trigger events.
	HTTP *HTTPEventData
	// Cron is non-nil for cron trigger events.
	Cron *CronEventData
	// Event is non-nil for event bus trigger events.
	Event *EventBusData

	config      map[string]interface{}
	logs        []logEntry
	replyBlocks []msgBlock
	httpResp    *httpResponseData
}

// FileRef is a lightweight reference to a stored file.
type FileRef struct {
	ID       string `json:"id" msgpack:"id"`
	Name     string `json:"name" msgpack:"name"`
	MIMEType string `json:"mime_type" msgpack:"mime_type"`
	Size     int64  `json:"size" msgpack:"size"`
	FileType string `json:"file_type" msgpack:"file_type"`
}

// MessengerData contains messenger command data.
type MessengerData struct {
	UserID      int64
	ChannelType string
	ChatID      string
	CommandName string
	Params      map[string]string
	Locale      string
	Files       []FileRef // file references attached to this message
}

// HTTPEventData contains HTTP request details for HTTP triggers.
type HTTPEventData struct {
	Method     string
	Path       string
	Query      map[string]string
	Headers    map[string]string
	Body       string
	RemoteAddr string
}

// CronEventData contains details for cron triggers.
type CronEventData struct {
	ScheduleName string
	FireTime     int64
}

// EventBusData contains details for event bus triggers.
type EventBusData struct {
	Topic   string
	Payload []byte
	Source  string
}

// Reply sets the reply message for messenger commands. Supports rich content
// and built-in localization via [Message].
//
//	// Simple text
//	ctx.Reply(wasmplugin.NewMessage("Готово!"))
//
//	// Localized
//	ctx.Reply(wasmplugin.NewLocalizedMessage(catalog.L("done")))
//
//	// Rich content with file
//	ctx.Reply(wasmplugin.NewMessage("Вот расписание").File(ref, "schedule.pdf"))
func (ctx *EventContext) Reply(msg Message) {
	ctx.replyBlocks = msg.blocks
}

// SetHTTPResponse sets the HTTP response for an HTTP trigger.
func (ctx *EventContext) SetHTTPResponse(statusCode int, headers map[string]string, body string) {
	ctx.httpResp = &httpResponseData{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       body,
	}
}

// JSON is a convenience method for replying with JSON to an HTTP trigger.
func (ctx *EventContext) JSON(statusCode int, v interface{}) {
	data, _ := json.Marshal(v)
	ctx.SetHTTPResponse(statusCode, map[string]string{"Content-Type": "application/json"}, string(data))
}

// Log records an informational log message.
func (ctx *EventContext) Log(msg string) {
	ctx.logs = append(ctx.logs, logEntry{Level: "info", Msg: msg})
}

// LogError records an error log message.
func (ctx *EventContext) LogError(msg string) {
	ctx.logs = append(ctx.logs, logEntry{Level: "error", Msg: msg})
}

// Config returns a config value by key, or the fallback if not set.
func (ctx *EventContext) Config(key string, fallback string) string {
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

// Files returns the file references attached to this messenger event.
func (ctx *EventContext) Files() []FileRef {
	if ctx.Messenger == nil {
		return nil
	}
	return ctx.Messenger.Files
}

// HasFiles returns true if the event has attached files.
func (ctx *EventContext) HasFiles() bool {
	return ctx.Messenger != nil && len(ctx.Messenger.Files) > 0
}

// Param returns a command parameter by key (convenience for messenger events).
func (ctx *EventContext) Param(key string) string {
	if ctx.Messenger == nil {
		return ""
	}
	return ctx.Messenger.Params[key]
}

// Locale returns the user's locale (convenience for messenger events).
func (ctx *EventContext) Locale() string {
	if ctx.Messenger != nil {
		return ctx.Messenger.Locale
	}
	return "en"
}

// MigrateContext provides version information and data access for plugin data migrations.
type MigrateContext struct {
	// OldVersion is the previously loaded plugin version.
	OldVersion string
	// NewVersion is the version being loaded now.
	NewVersion string
}
