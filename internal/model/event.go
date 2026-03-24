package model

import "encoding/json"

type TriggerType string

const (
	TriggerHTTP      TriggerType = "http"
	TriggerCron      TriggerType = "cron"
	TriggerEvent     TriggerType = "event"
	TriggerMessenger TriggerType = "messenger"
)

type Event struct {
	ID          string          `json:"id"`
	TriggerType TriggerType     `json:"trigger_type"`
	TriggerName string          `json:"trigger_name"`
	PluginID    string          `json:"plugin_id"`
	Timestamp   int64           `json:"timestamp"`
	Data        json.RawMessage `json:"data"`
}

func (e Event) Messenger() (*MessengerTriggerData, error) {
	var data MessengerTriggerData
	if err := json.Unmarshal(e.Data, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

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

type EventResponse struct {
	Status   string          `json:"status,omitempty"`
	Error    string          `json:"error,omitempty"`
	Reply    string          `json:"reply,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
	Logs     []LogEntry      `json:"logs,omitempty"`
	Messages []MessageEntry  `json:"messages,omitempty"`
}

type LogEntry struct {
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

type MessageEntry struct {
	ChannelType ChannelType       `json:"channel_type,omitempty"`
	ChatID      string            `json:"chat_id,omitempty"`
	UserID      GlobalUserID      `json:"user_id,omitempty"`
	Text        string            `json:"text,omitempty"`
	Texts       map[string]string `json:"texts,omitempty"`
}

type MessengerTriggerData struct {
	UserID      GlobalUserID `json:"user_id"`
	ChannelType ChannelType  `json:"channel_type"`
	ChatID      string       `json:"chat_id"`
	CommandName string       `json:"command_name"`
	Params      OptionMap    `json:"params,omitempty"`
	Locale      string       `json:"locale"`
}

type HTTPTriggerData struct {
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Query      map[string]string `json:"query,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	RemoteAddr string            `json:"remote_addr,omitempty"`
}

type HTTPResponseData struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body"`
}

type CronTriggerData struct {
	ScheduleName string `json:"schedule_name"`
	FireTime     int64  `json:"fire_time"`
}

type EventTriggerData struct {
	Topic   string          `json:"topic"`
	Payload json.RawMessage `json:"payload"`
	Source  string          `json:"source"`
}
