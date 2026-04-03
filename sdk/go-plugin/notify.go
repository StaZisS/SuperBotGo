//go:build wasip1

package wasmplugin

import "fmt"

//go:wasmimport env notify_user
func _notify_user(offset, length uint32) uint64

//go:wasmimport env notify_chat
func _notify_chat(offset, length uint32) uint64

//go:wasmimport env notify_students
func _notify_students(offset, length uint32) uint64

const (
	PriorityLow      = 0 // Informational, silent outside work hours
	PriorityNormal   = 1 // Standard notification with sound
	PriorityHigh     = 2 // Important — auto-mention user
	PriorityCritical = 3 // Urgent — mention, all channels, never silent
)

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

type notifyStudentsReq struct {
	Scope    string     `msgpack:"scope"`
	TargetID int64      `msgpack:"target_id"`
	Blocks   []msgBlock `msgpack:"blocks"`
	Priority int        `msgpack:"priority"`
}

type notifyResp struct {
	OK    bool   `msgpack:"ok"`
	Error string `msgpack:"error,omitempty"`
}

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

// NotifyStudents returns a builder for sending a priority-aware notification
// to students within a university hierarchy scope.
//
// Usage:
//
//	ctx.NotifyStudents().
//	    Stream(streamID).
//	    Message(wasmplugin.NewMessage("Пары завтра отменены")).
//	    Priority(wasmplugin.PriorityHigh).
//	    Send()
func (ctx *EventContext) NotifyStudents() *StudentNotifyBuilder {
	return &StudentNotifyBuilder{priority: PriorityNormal}
}

// StudentNotifyBuilder constructs a student notification via method chaining.
type StudentNotifyBuilder struct {
	scope    string
	targetID int64
	msg      Message
	priority int
}

// Faculty targets all students of the given faculty.
func (b *StudentNotifyBuilder) Faculty(id int64) *StudentNotifyBuilder {
	b.scope = "faculty"
	b.targetID = id
	return b
}

// Department targets all students of the given department.
func (b *StudentNotifyBuilder) Department(id int64) *StudentNotifyBuilder {
	b.scope = "department"
	b.targetID = id
	return b
}

// Program targets all students of the given program.
func (b *StudentNotifyBuilder) Program(id int64) *StudentNotifyBuilder {
	b.scope = "program"
	b.targetID = id
	return b
}

// Stream targets all students of the given stream.
func (b *StudentNotifyBuilder) Stream(id int64) *StudentNotifyBuilder {
	b.scope = "stream"
	b.targetID = id
	return b
}

// Group targets all students of the given study group.
func (b *StudentNotifyBuilder) Group(id int64) *StudentNotifyBuilder {
	b.scope = "group"
	b.targetID = id
	return b
}

// Subgroup targets all students of the given subgroup.
func (b *StudentNotifyBuilder) Subgroup(id int64) *StudentNotifyBuilder {
	b.scope = "subgroup"
	b.targetID = id
	return b
}

// Message sets the notification message.
func (b *StudentNotifyBuilder) Message(msg Message) *StudentNotifyBuilder {
	b.msg = msg
	return b
}

// Priority sets the notification priority. Defaults to PriorityNormal.
func (b *StudentNotifyBuilder) Priority(p int) *StudentNotifyBuilder {
	b.priority = p
	return b
}

// Send executes the notification. Returns an error if scope or text is not set.
func (b *StudentNotifyBuilder) Send() error {
	if b.scope == "" {
		return fmt.Errorf("notify_students: scope not set (use Stream, Group, Subgroup, etc.)")
	}
	if b.msg.IsEmpty() {
		return fmt.Errorf("notify_students: message not set")
	}

	var resp notifyResp
	if err := callHostWithResult(_notify_students, notifyStudentsReq{
		Scope:    b.scope,
		TargetID: b.targetID,
		Blocks:   b.msg.blocks,
		Priority: b.priority,
	}, &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return fmt.Errorf("notify_students: %s", resp.Error)
	}
	return nil
}
