package model

import "encoding/json"

// TriggerType identifies the source of an event.
type TriggerType string

const (
	TriggerHTTP      TriggerType = "http"
	TriggerCron      TriggerType = "cron"
	TriggerEvent     TriggerType = "event"
	TriggerMessenger TriggerType = "messenger"
)

// Event is the unified envelope sent to plugins for all trigger types.
type Event struct {
	ID          string          `json:"id"`
	TriggerType TriggerType     `json:"trigger_type"`
	TriggerName string          `json:"trigger_name"`
	PluginID    string          `json:"plugin_id"`
	Timestamp   int64           `json:"timestamp"`
	Data        json.RawMessage `json:"data"`
}

// Messenger parses the messenger-specific data from an event.
func (e Event) Messenger() (*MessengerTriggerData, error) {
	var data MessengerTriggerData
	if err := json.Unmarshal(e.Data, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// NewMessengerEvent creates an Event from a CommandRequest.
func NewMessengerEvent(req CommandRequest, pluginID string) Event {
	data, _ := json.Marshal(MessengerTriggerData{
		UserID:      req.UserID,
		ChannelType: req.ChannelType,
		ChatID:      req.ChatID,
		CommandName: req.CommandName,
		Params:      req.Params,
		Locale:      req.Locale,
	})
	return Event{
		TriggerType: TriggerMessenger,
		TriggerName: req.CommandName,
		PluginID:    pluginID,
		Data:        data,
	}
}

// EventResponse is the plugin's response to an event.
type EventResponse struct {
	Status   string          `json:"status,omitempty"`
	Error    string          `json:"error,omitempty"`
	Reply    string          `json:"reply,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
	Logs     []LogEntry      `json:"logs,omitempty"`
	Messages []MessageEntry  `json:"messages,omitempty"`
}

// LogEntry is a single log line from a plugin.
type LogEntry struct {
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

// MessageEntry is an outbound message to a messenger chat.
type MessageEntry struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

// MessengerTriggerData is Event.Data for TriggerMessenger.
type MessengerTriggerData struct {
	UserID      GlobalUserID `json:"user_id"`
	ChannelType ChannelType  `json:"channel_type"`
	ChatID      string       `json:"chat_id"`
	CommandName string       `json:"command_name"`
	Params      OptionMap    `json:"params,omitempty"`
	Locale      string       `json:"locale"`
}

// HTTPTriggerData is Event.Data for TriggerHTTP.
type HTTPTriggerData struct {
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Query      map[string]string `json:"query,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	RemoteAddr string            `json:"remote_addr,omitempty"`
}

// HTTPResponseData is EventResponse.Data for TriggerHTTP.
type HTTPResponseData struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body"`
}

// CronTriggerData is Event.Data for TriggerCron.
type CronTriggerData struct {
	ScheduleName string `json:"schedule_name"`
	FireTime     int64  `json:"fire_time"`
}

// EventTriggerData is Event.Data for TriggerEvent.
type EventTriggerData struct {
	Topic   string          `json:"topic"`
	Payload json.RawMessage `json:"payload"`
	Source  string          `json:"source"`
}
