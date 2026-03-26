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

	config     map[string]interface{}
	logs       []logEntry
	messages   []messageEntry
	reply      string
	replyTexts map[string]string
	httpResp   *httpResponseData
}

// MessengerData contains messenger command data.
type MessengerData struct {
	UserID      int64
	ChannelType string
	ChatID      string
	CommandName string
	Params      map[string]string
	Locale      string
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

// Reply sets the text reply for messenger commands.
func (ctx *EventContext) Reply(text string) {
	ctx.reply = text
}

// ReplyLocalized sets a localized reply for messenger commands. The texts map
// is keyed by locale code (e.g. "en", "ru"). The host resolves the target
// locale from the user's or chat's settings. Use with [Catalog.L]:
//
//	ctx.ReplyLocalized(cat.L("schedule_header", "Building", building))
func (ctx *EventContext) ReplyLocalized(texts map[string]string) {
	ctx.replyTexts = texts
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

// SendMessage queues a message to be sent to the given messenger chat.
func (ctx *EventContext) SendMessage(chatID string, text string) {
	ctx.messages = append(ctx.messages, messageEntry{ChatID: chatID, Text: text})
}

// SendLocalizedMessage queues a localized message to a chat. The host picks
// the text matching the chat's configured locale. The texts map should be
// keyed by locale code (e.g. "en", "ru").
func (ctx *EventContext) SendLocalizedMessage(chatID string, texts map[string]string) {
	ctx.messages = append(ctx.messages, messageEntry{ChatID: chatID, Texts: texts})
}

// SendLocalizedToUser queues a localized DM to a user. The host resolves the
// user's locale and delivers to their primary channel. The texts map should be
// keyed by locale code (e.g. "en", "ru").
func (ctx *EventContext) SendLocalizedToUser(userID int64, texts map[string]string) {
	ctx.messages = append(ctx.messages, messageEntry{UserID: userID, Texts: texts})
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

// ---------------------------------------------------------------------------
// MigrateContext — passed to Plugin.Migrate during version upgrades
// ---------------------------------------------------------------------------

// MigrateContext provides version information and data access for plugin
// data migrations. It is passed to the Plugin.Migrate handler when the host
// detects a version change during reload.
type MigrateContext struct {
	// OldVersion is the previously loaded plugin version.
	OldVersion string
	// NewVersion is the version being loaded now.
	NewVersion string
}
