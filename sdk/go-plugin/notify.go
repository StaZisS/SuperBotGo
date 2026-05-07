//go:build wasip1

package wasmplugin

import "fmt"

//go:wasmimport env notify_user
func _notify_user(offset, length uint32) uint64

//go:wasmimport env notify_users
func _notify_users(offset, length uint32) uint64

//go:wasmimport env notify_teacher
func _notify_teacher(offset, length uint32) uint64

//go:wasmimport env notify_chat
func _notify_chat(offset, length uint32) uint64

//go:wasmimport env notify_students
func _notify_students(offset, length uint32) uint64

const (
	PriorityLow      = 0 // Informational, delayed outside work hours
	PriorityNormal   = 1 // Standard notification with sound
	PriorityHigh     = 2 // Important — auto-mention user
	PriorityCritical = 3 // Urgent — mention, all channels, never delayed
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

type notifyUsersReq struct {
	UserIDs  []int64    `msgpack:"user_ids"`
	Blocks   []msgBlock `msgpack:"blocks"`
	Priority int        `msgpack:"priority"`
}

type notifyTeacherReq struct {
	TeacherPositionID int64      `msgpack:"teacher_position_id,omitempty"`
	PersonID          int64      `msgpack:"person_id,omitempty"`
	ExternalID        string     `msgpack:"external_id,omitempty"`
	Blocks            []msgBlock `msgpack:"blocks"`
	Priority          int        `msgpack:"priority"`
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

// NotifyRecipient sends one rich notification to a single user.
func (ctx *EventContext) NotifyRecipient(userID int64, msg Message, priority int) error {
	return ctx.NotifyUsers([]int64{userID}, msg, priority)
}

// NotifyTeacher sends one rich notification to a teacher by teacher_position.id.
func (ctx *EventContext) NotifyTeacher(teacherPositionID int64, msg Message, priority int) error {
	return ctx.notifyTeacher(notifyTeacherReq{TeacherPositionID: teacherPositionID, Blocks: msg.blocks, Priority: priority})
}

// NotifyTeacherPerson sends one rich notification to a teacher by persons.id.
func (ctx *EventContext) NotifyTeacherPerson(personID int64, msg Message, priority int) error {
	return ctx.notifyTeacher(notifyTeacherReq{PersonID: personID, Blocks: msg.blocks, Priority: priority})
}

// NotifyTeacherExternalID sends one rich notification to a teacher by persons.external_id.
func (ctx *EventContext) NotifyTeacherExternalID(externalID string, msg Message, priority int) error {
	return ctx.notifyTeacher(notifyTeacherReq{ExternalID: externalID, Blocks: msg.blocks, Priority: priority})
}

func (ctx *EventContext) notifyTeacher(req notifyTeacherReq) error {
	if req.TeacherPositionID <= 0 && req.PersonID <= 0 && req.ExternalID == "" {
		return fmt.Errorf("notify_teacher: teacher reference not set")
	}
	if len(req.Blocks) == 0 {
		return fmt.Errorf("notify_teacher: message not set")
	}

	var resp notifyResp
	if err := callHostWithResult(_notify_teacher, req, &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return fmt.Errorf("notify_teacher: %s", resp.Error)
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

// NotifyUsers sends one rich notification to each listed user.
func (ctx *EventContext) NotifyUsers(userIDs []int64, msg Message, priority int) error {
	if len(userIDs) == 0 {
		return fmt.Errorf("notify_users: no user IDs provided")
	}
	if msg.IsEmpty() {
		return fmt.Errorf("notify_users: message not set")
	}

	var resp notifyResp
	if err := callHostWithResult(_notify_users, notifyUsersReq{
		UserIDs:  userIDs,
		Blocks:   msg.blocks,
		Priority: priority,
	}, &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return fmt.Errorf("notify_users: %s", resp.Error)
	}
	return nil
}

// NotifyRecipients returns a builder for sending a rich notification to specific users.
//
// Usage:
//
//	ctx.NotifyRecipients().
//	    Teacher(teacherUserID).
//	    Message(wasmplugin.NewMessage("Проверьте замену в расписании")).
//	    Priority(wasmplugin.PriorityHigh).
//	    Send()
func (ctx *EventContext) NotifyRecipients() *RecipientNotifyBuilder {
	return &RecipientNotifyBuilder{
		ctx:      ctx,
		priority: PriorityNormal,
		seen:     make(map[int64]struct{}),
	}
}

// RecipientNotifyBuilder constructs a direct user notification via method chaining.
type RecipientNotifyBuilder struct {
	ctx      *EventContext
	userIDs  []int64
	teachers []notifyTeacherReq
	msg      Message
	priority int
	seen     map[int64]struct{}
}

// User adds one global user ID as a recipient.
func (b *RecipientNotifyBuilder) User(userID int64) *RecipientNotifyBuilder {
	if userID <= 0 {
		return b
	}
	if _, ok := b.seen[userID]; ok {
		return b
	}
	b.seen[userID] = struct{}{}
	b.userIDs = append(b.userIDs, userID)
	return b
}

// Teacher adds a teacher by teacher_position.id.
func (b *RecipientNotifyBuilder) Teacher(teacherPositionID int64) *RecipientNotifyBuilder {
	if teacherPositionID <= 0 {
		return b
	}
	b.teachers = append(b.teachers, notifyTeacherReq{TeacherPositionID: teacherPositionID})
	return b
}

// TeacherPerson adds a teacher by persons.id.
func (b *RecipientNotifyBuilder) TeacherPerson(personID int64) *RecipientNotifyBuilder {
	if personID <= 0 {
		return b
	}
	b.teachers = append(b.teachers, notifyTeacherReq{PersonID: personID})
	return b
}

// TeacherExternalID adds a teacher by persons.external_id.
func (b *RecipientNotifyBuilder) TeacherExternalID(externalID string) *RecipientNotifyBuilder {
	if externalID == "" {
		return b
	}
	b.teachers = append(b.teachers, notifyTeacherReq{ExternalID: externalID})
	return b
}

// Users adds multiple global user IDs as recipients.
func (b *RecipientNotifyBuilder) Users(userIDs ...int64) *RecipientNotifyBuilder {
	for _, userID := range userIDs {
		b.User(userID)
	}
	return b
}

// Message sets the notification message.
func (b *RecipientNotifyBuilder) Message(msg Message) *RecipientNotifyBuilder {
	b.msg = msg
	return b
}

// Priority sets the notification priority. Defaults to PriorityNormal.
func (b *RecipientNotifyBuilder) Priority(p int) *RecipientNotifyBuilder {
	b.priority = p
	return b
}

// Send executes the notification. Returns an error if recipients or message are not set.
func (b *RecipientNotifyBuilder) Send() error {
	if len(b.userIDs) == 0 && len(b.teachers) == 0 {
		return fmt.Errorf("notify_recipients: no recipients provided")
	}
	if b.msg.IsEmpty() {
		return fmt.Errorf("notify_recipients: message not set")
	}

	if len(b.userIDs) > 0 {
		if err := b.ctx.NotifyUsers(b.userIDs, b.msg, b.priority); err != nil {
			return err
		}
	}
	for _, teacher := range b.teachers {
		teacher.Blocks = b.msg.blocks
		teacher.Priority = b.priority
		if err := b.ctx.notifyTeacher(teacher); err != nil {
			return err
		}
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
