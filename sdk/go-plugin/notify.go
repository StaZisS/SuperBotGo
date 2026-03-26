//go:build wasip1

package wasmplugin

import "fmt"

// ---------------------------------------------------------------------------
// Notification host function imports (wasm -> host)
// ---------------------------------------------------------------------------

//go:wasmimport env notify_user
func _notify_user(offset, length uint32) uint64

//go:wasmimport env notify_chat
func _notify_chat(offset, length uint32) uint64

//go:wasmimport env notify_project
func _notify_project(offset, length uint32) uint64

// ---------------------------------------------------------------------------
// Priority constants
// ---------------------------------------------------------------------------

const (
	PriorityLow      = 0 // Informational, silent outside work hours
	PriorityNormal   = 1 // Standard notification with sound
	PriorityHigh     = 2 // Important — auto-mention user
	PriorityCritical = 3 // Urgent — mention, all channels, never silent
)

// ---------------------------------------------------------------------------
// Request / response types
// ---------------------------------------------------------------------------

type notifyUserReq struct {
	UserID   int64  `msgpack:"user_id"`
	Text     string `msgpack:"text"`
	Priority int    `msgpack:"priority"`
}

type notifyChatReq struct {
	ChannelType string `msgpack:"channel_type"`
	ChatID      string `msgpack:"chat_id"`
	Text        string `msgpack:"text"`
	Priority    int    `msgpack:"priority"`
}

type notifyProjectReq struct {
	ProjectID int64  `msgpack:"project_id"`
	Text      string `msgpack:"text"`
	Priority  int    `msgpack:"priority"`
}

type notifyResp struct {
	OK    bool   `msgpack:"ok"`
	Error string `msgpack:"error,omitempty"`
}

// ---------------------------------------------------------------------------
// Public API on EventContext
// ---------------------------------------------------------------------------

// NotifyUser sends a priority-aware notification to a user.
// Priority: PriorityLow (0), PriorityNormal (1), PriorityHigh (2), PriorityCritical (3).
func (ctx *EventContext) NotifyUser(userID int64, text string, priority int) error {
	var resp notifyResp
	if err := callHostWithResult(_notify_user, notifyUserReq{
		UserID:   userID,
		Text:     text,
		Priority: priority,
	}, &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return fmt.Errorf("notify_user: %s", resp.Error)
	}
	return nil
}

// NotifyChat sends a priority-aware notification to a specific chat.
func (ctx *EventContext) NotifyChat(channelType, chatID, text string, priority int) error {
	var resp notifyResp
	if err := callHostWithResult(_notify_chat, notifyChatReq{
		ChannelType: channelType,
		ChatID:      chatID,
		Text:        text,
		Priority:    priority,
	}, &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return fmt.Errorf("notify_chat: %s", resp.Error)
	}
	return nil
}

// NotifyProject sends a priority-aware notification to all chats bound to a project.
func (ctx *EventContext) NotifyProject(projectID int64, text string, priority int) error {
	var resp notifyResp
	if err := callHostWithResult(_notify_project, notifyProjectReq{
		ProjectID: projectID,
		Text:      text,
		Priority:  priority,
	}, &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return fmt.Errorf("notify_project: %s", resp.Error)
	}
	return nil
}
