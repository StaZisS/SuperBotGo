package notification

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/model"
)

type UserService interface {
	GetUser(ctx context.Context, id model.GlobalUserID) (*model.GlobalUser, error)
}

type StudentResolver interface {
	ResolveStudentUsers(ctx context.Context, scope string, targetID int64) ([]model.GlobalUserID, error)
}

type NotifyAPI struct {
	adapters *channel.AdapterRegistry
	users    UserService
	prefs    PrefsRepository
	students StudentResolver

	mu        sync.Mutex
	scheduled []scheduledMsg
}

type scheduledMsg struct {
	userID    model.GlobalUserID
	msg       model.Message
	priority  model.NotifyPriority
	sendAt    time.Time
	createdAt time.Time
}

func NewNotifyAPI(
	adapters *channel.AdapterRegistry,
	users UserService,
	prefs PrefsRepository,
	students StudentResolver,
) *NotifyAPI {
	api := &NotifyAPI{
		adapters: adapters,
		users:    users,
		prefs:    prefs,
		students: students,
	}

	go api.startWorker(context.Background())

	return api
}

func (n *NotifyAPI) NotifyUser(
	ctx context.Context,
	userID model.GlobalUserID,
	msg model.Message,
	priority model.NotifyPriority,
) error {

	user, err := n.users.GetUser(ctx, userID)
	if err != nil || user == nil {
		return fmt.Errorf("user not found: %w", err)
	}

	prefs, err := n.prefs.GetPrefs(ctx, userID)
	if err != nil {
		return err
	}
	if prefs == nil {
		prefs = defaultPrefs(userID, user.PrimaryChannel)
	}

	if priority == model.PriorityLow && !isWithinWorkHours(prefs) {
		n.schedule(userID, msg, priority, prefs)
		return nil
	}

	opts := n.buildSendOptions(prefs, priority)

	return n.sendToUser(ctx, user, msg, priority, opts)
}

func (n *NotifyAPI) sendToUser(
	ctx context.Context,
	user *model.GlobalUser,
	msg model.Message,
	priority model.NotifyPriority,
	opts model.SendOptions,
) error {

	if priority == model.PriorityCritical {
		for _, acc := range user.Accounts {
			_ = n.adapters.SendToUserWithOpts(
				ctx,
				acc.ChannelType,
				acc.ChannelUserID,
				msg,
				opts,
			)
		}
		return nil
	}

	ch, id := n.resolveChannel(user)

	return n.adapters.SendToUserWithOpts(ctx, ch, id, msg, opts)
}

func (n *NotifyAPI) schedule(
	userID model.GlobalUserID,
	msg model.Message,
	priority model.NotifyPriority,
	prefs *model.NotificationPrefs,
) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.scheduled = append(n.scheduled, scheduledMsg{
		userID:    userID,
		msg:       msg,
		priority:  priority,
		sendAt:    nextWorkTime(prefs),
		createdAt: time.Now(),
	})

	slog.Info("message scheduled",
		slog.Int64("user_id", int64(userID)),
		slog.Time("send_at", nextWorkTime(prefs)),
	)
}

func (n *NotifyAPI) startWorker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)

	for {
		select {
		case <-ticker.C:
			n.process(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (n *NotifyAPI) process(ctx context.Context) {
	now := time.Now()

	n.mu.Lock()
	defer n.mu.Unlock()

	var rest []scheduledMsg

	for _, m := range n.scheduled {
		if m.sendAt.Before(now) {
			_ = n.NotifyUser(ctx, m.userID, m.msg, m.priority)
		} else {
			rest = append(rest, m)
		}
	}

	n.scheduled = rest
}

func nextWorkTime(prefs *model.NotificationPrefs) time.Time {
	now := time.Now()

	if prefs.WorkHoursStart == nil {
		return now.Add(1 * time.Minute)
	}

	loc := time.UTC
	if prefs.Timezone != "" {
		if l, err := time.LoadLocation(prefs.Timezone); err == nil {
			loc = l
		}
	}

	start := time.Date(now.Year(), now.Month(), now.Day(),
		*prefs.WorkHoursStart, 0, 0, 0, loc)

	if now.Before(start) {
		return start
	}

	return start.Add(24 * time.Hour)
}

func isWithinWorkHours(prefs *model.NotificationPrefs) bool {
	if prefs.WorkHoursStart == nil || prefs.WorkHoursEnd == nil {
		return true
	}

	now := time.Now().UTC().Hour()
	start := *prefs.WorkHoursStart
	end := *prefs.WorkHoursEnd

	if start <= end {
		return now >= start && now < end
	}
	return now >= start || now < end
}

func (n *NotifyAPI) buildSendOptions(
	prefs *model.NotificationPrefs,
	priority model.NotifyPriority,
) model.SendOptions {

	return model.SendOptions{
		Silent: priority == model.PriorityLow,
	}
}

func (n *NotifyAPI) resolveChannel(user *model.GlobalUser) (model.ChannelType, model.PlatformUserID) {
	if len(user.Accounts) > 0 {
		acc := user.Accounts[0]
		return acc.ChannelType, acc.ChannelUserID
	}
	return user.PrimaryChannel, ""
}

func defaultPrefs(userID model.GlobalUserID, primary model.ChannelType) *model.NotificationPrefs {
	return &model.NotificationPrefs{
		GlobalUserID:    userID,
		ChannelPriority: []model.ChannelType{primary},
		Timezone:        "UTC",
	}
}
