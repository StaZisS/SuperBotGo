package notification

import (
	"context"
	"errors"
	"testing"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/model"
)

// --- Mock UserService ---

type mockUserService struct {
	getUserFn func(ctx context.Context, id model.GlobalUserID) (*model.GlobalUser, error)
}

func (m *mockUserService) GetUser(ctx context.Context, id model.GlobalUserID) (*model.GlobalUser, error) {
	return m.getUserFn(ctx, id)
}

// --- Mock PrefsRepository ---

type mockPrefsRepo struct {
	getPrefsFn  func(ctx context.Context, userID model.GlobalUserID) (*model.NotificationPrefs, error)
	savePrefsFn func(ctx context.Context, prefs *model.NotificationPrefs) error
}

func (m *mockPrefsRepo) GetPrefs(ctx context.Context, userID model.GlobalUserID) (*model.NotificationPrefs, error) {
	return m.getPrefsFn(ctx, userID)
}

func (m *mockPrefsRepo) SavePrefs(ctx context.Context, prefs *model.NotificationPrefs) error {
	if m.savePrefsFn != nil {
		return m.savePrefsFn(ctx, prefs)
	}
	return nil
}

// --- Mock StudentResolver ---

type mockStudentResolver struct {
	resolveFn func(ctx context.Context, scope string, targetID int64) ([]model.GlobalUserID, error)
}

func (m *mockStudentResolver) ResolveStudentUsers(ctx context.Context, scope string, targetID int64) ([]model.GlobalUserID, error) {
	return m.resolveFn(ctx, scope, targetID)
}

// --- Mock ChannelAdapter ---

type mockAdapter struct {
	channelType  model.ChannelType
	sendToUserFn func(ctx context.Context, platformUserID model.PlatformUserID, msg model.Message) error
	sendToChatFn func(ctx context.Context, chatID string, msg model.Message) error
}

func (m *mockAdapter) Type() model.ChannelType {
	return m.channelType
}

func (m *mockAdapter) SendToUser(ctx context.Context, platformUserID model.PlatformUserID, msg model.Message) error {
	if m.sendToUserFn != nil {
		return m.sendToUserFn(ctx, platformUserID, msg)
	}
	return nil
}

func (m *mockAdapter) SendToChat(ctx context.Context, chatID string, msg model.Message) error {
	if m.sendToChatFn != nil {
		return m.sendToChatFn(ctx, chatID, msg)
	}
	return nil
}

// --- Helper to build AdapterRegistry ---

func newRegistryWithAdapters(adapters ...channel.ChannelAdapter) *channel.AdapterRegistry {
	reg := channel.NewAdapterRegistry()
	for _, a := range adapters {
		reg.Register(a)
	}
	return reg
}

// --- Tests for resolveChannel ---

func TestResolveChannel_ByPriority(t *testing.T) {
	t.Parallel()

	user := &model.GlobalUser{
		ID:             1,
		PrimaryChannel: model.ChannelTelegram,
		Accounts: []model.ChannelAccount{
			{ChannelType: model.ChannelTelegram, ChannelUserID: "tg-1"},
			{ChannelType: model.ChannelDiscord, ChannelUserID: "dc-1"},
		},
	}
	prefs := &model.NotificationPrefs{
		ChannelPriority: []model.ChannelType{model.ChannelDiscord, model.ChannelTelegram},
	}

	api := &NotifyAPI{}
	ch, pid := api.resolveChannel(user, prefs)

	if ch != model.ChannelDiscord {
		t.Errorf("channel = %s, want %s", ch, model.ChannelDiscord)
	}
	if pid != "dc-1" {
		t.Errorf("platformUserID = %s, want dc-1", pid)
	}
}

func TestResolveChannel_FallbackPrimary(t *testing.T) {
	t.Parallel()

	user := &model.GlobalUser{
		ID:             1,
		PrimaryChannel: model.ChannelTelegram,
		Accounts: []model.ChannelAccount{
			{ChannelType: model.ChannelTelegram, ChannelUserID: "tg-1"},
		},
	}
	// Prefs request Discord first, but user has no Discord account.
	prefs := &model.NotificationPrefs{
		ChannelPriority: []model.ChannelType{model.ChannelDiscord},
	}

	api := &NotifyAPI{}
	ch, pid := api.resolveChannel(user, prefs)

	if ch != model.ChannelTelegram {
		t.Errorf("channel = %s, want %s (primary fallback)", ch, model.ChannelTelegram)
	}
	if pid != "tg-1" {
		t.Errorf("platformUserID = %s, want tg-1", pid)
	}
}

func TestResolveChannel_FallbackFirstAccount(t *testing.T) {
	t.Parallel()

	user := &model.GlobalUser{
		ID:             1,
		PrimaryChannel: model.ChannelDiscord, // primary is Discord, but no Discord account
		Accounts: []model.ChannelAccount{
			{ChannelType: model.ChannelTelegram, ChannelUserID: "tg-1"},
		},
	}
	prefs := &model.NotificationPrefs{
		ChannelPriority: []model.ChannelType{model.ChannelDiscord},
	}

	api := &NotifyAPI{}
	ch, pid := api.resolveChannel(user, prefs)

	if ch != model.ChannelTelegram {
		t.Errorf("channel = %s, want %s (first account fallback)", ch, model.ChannelTelegram)
	}
	if pid != "tg-1" {
		t.Errorf("platformUserID = %s, want tg-1", pid)
	}
}

// --- Tests for isWithinWorkHours ---

func TestIsWithinWorkHours_NilHours(t *testing.T) {
	t.Parallel()

	prefs := &model.NotificationPrefs{
		WorkHoursStart: nil,
		WorkHoursEnd:   nil,
	}

	if !isWithinWorkHours(prefs) {
		t.Error("expected true when work hours are nil (always within)")
	}
}

func TestIsWithinWorkHours_Within(t *testing.T) {
	t.Parallel()

	start, end := 0, 24 // covers all hours
	prefs := &model.NotificationPrefs{
		WorkHoursStart: &start,
		WorkHoursEnd:   &end,
		Timezone:       "UTC",
	}

	if !isWithinWorkHours(prefs) {
		t.Error("expected true when work hours span 0-24 (all day)")
	}
}

func TestIsWithinWorkHours_Outside(t *testing.T) {
	t.Parallel()

	// Use a narrow window that certainly does not include the current UTC hour.
	// By setting start=end, the range is empty for the normal case (start <= end means hour >= start && hour < end).
	start, end := 25, 25 // impossible hour, will never match
	prefs := &model.NotificationPrefs{
		WorkHoursStart: &start,
		WorkHoursEnd:   &end,
		Timezone:       "UTC",
	}

	// start <= end (25 <= 25), so condition is hour >= 25 && hour < 25, which is always false.
	if isWithinWorkHours(prefs) {
		t.Error("expected false when work hours window is empty")
	}
}

func TestIsWithinWorkHours_OvernightWrap(t *testing.T) {
	t.Parallel()

	// Overnight wrap: start > end means hour >= start OR hour < end.
	// start=22, end=6 covers 22:00-05:59 — always includes some hours.
	start, end := 22, 6
	prefs := &model.NotificationPrefs{
		WorkHoursStart: &start,
		WorkHoursEnd:   &end,
		Timezone:       "UTC",
	}

	// This tests the overnight logic path. Since current hour is dynamic,
	// we just verify it does not panic and returns a boolean.
	_ = isWithinWorkHours(prefs)
}

// --- Tests for buildSendOptions ---

func TestBuildSendOptions_LowOutsideWorkHours(t *testing.T) {
	t.Parallel()

	// Empty work hours window so isWithinWorkHours returns false.
	start, end := 25, 25
	prefs := &model.NotificationPrefs{
		WorkHoursStart: &start,
		WorkHoursEnd:   &end,
		Timezone:       "UTC",
	}

	api := &NotifyAPI{}
	opts := api.buildSendOptions(prefs, model.PriorityLow)

	if !opts.Silent {
		t.Error("expected Silent=true for low priority outside work hours")
	}
}

func TestBuildSendOptions_MuteMentions(t *testing.T) {
	t.Parallel()

	prefs := &model.NotificationPrefs{
		MuteMentions: true,
	}

	api := &NotifyAPI{}
	opts := api.buildSendOptions(prefs, model.PriorityNormal)

	if !opts.StripMentions {
		t.Error("expected StripMentions=true when MuteMentions is enabled and priority < Critical")
	}
}

func TestBuildSendOptions_CriticalIgnoresMuteMentions(t *testing.T) {
	t.Parallel()

	prefs := &model.NotificationPrefs{
		MuteMentions: true,
	}

	api := &NotifyAPI{}
	opts := api.buildSendOptions(prefs, model.PriorityCritical)

	if opts.StripMentions {
		t.Error("expected StripMentions=false for Critical priority even with MuteMentions")
	}
}

// --- Tests for NotifyChat ---

func TestNotifyChat(t *testing.T) {
	t.Parallel()

	var capturedChatID string
	var capturedMsg model.Message

	adapter := &mockAdapter{
		channelType: model.ChannelTelegram,
		sendToChatFn: func(_ context.Context, chatID string, msg model.Message) error {
			capturedChatID = chatID
			capturedMsg = msg
			return nil
		},
	}

	reg := newRegistryWithAdapters(adapter)
	api := NewNotifyAPI(reg, nil, nil, nil)

	msg := model.NewTextMessage("hello")
	err := api.NotifyChat(context.Background(), model.ChannelTelegram, "chat-42", msg, model.PriorityNormal)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedChatID != "chat-42" {
		t.Errorf("chatID = %s, want chat-42", capturedChatID)
	}
	if len(capturedMsg.Blocks) == 0 {
		t.Error("expected message blocks to be non-empty")
	}
}

func TestNotifyChat_LowPrioritySilent(t *testing.T) {
	t.Parallel()

	adapter := &mockAdapter{
		channelType: model.ChannelTelegram,
		sendToChatFn: func(_ context.Context, _ string, _ model.Message) error {
			return nil
		},
	}

	reg := newRegistryWithAdapters(adapter)
	api := NewNotifyAPI(reg, nil, nil, nil)

	// Low priority should result in Silent=true in SendOptions passed to SendToChatWithOpts.
	// The adapter does not implement SilentSender, so it falls through to normal send.
	err := api.NotifyChat(context.Background(), model.ChannelTelegram, "chat-1", model.NewTextMessage("low"), model.PriorityLow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Tests for NotifyUser ---

func TestNotifyUser_UserNotFound(t *testing.T) {
	t.Parallel()

	users := &mockUserService{
		getUserFn: func(_ context.Context, _ model.GlobalUserID) (*model.GlobalUser, error) {
			return nil, nil
		},
	}

	reg := newRegistryWithAdapters()
	api := NewNotifyAPI(reg, users, nil, nil)

	err := api.NotifyUser(context.Background(), 999, model.NewTextMessage("hello"), model.PriorityNormal)

	if err == nil {
		t.Fatal("expected error for user not found, got nil")
	}
}

func TestNotifyUser_GetUserError(t *testing.T) {
	t.Parallel()

	users := &mockUserService{
		getUserFn: func(_ context.Context, _ model.GlobalUserID) (*model.GlobalUser, error) {
			return nil, errors.New("db error")
		},
	}

	reg := newRegistryWithAdapters()
	api := NewNotifyAPI(reg, users, nil, nil)

	err := api.NotifyUser(context.Background(), 1, model.NewTextMessage("hello"), model.PriorityNormal)

	if err == nil {
		t.Fatal("expected error when GetUser fails, got nil")
	}
}

func TestNotifyUser_SendsToResolvedChannel(t *testing.T) {
	t.Parallel()

	var capturedPlatformID model.PlatformUserID

	adapter := &mockAdapter{
		channelType: model.ChannelDiscord,
		sendToUserFn: func(_ context.Context, pid model.PlatformUserID, _ model.Message) error {
			capturedPlatformID = pid
			return nil
		},
	}

	users := &mockUserService{
		getUserFn: func(_ context.Context, _ model.GlobalUserID) (*model.GlobalUser, error) {
			return &model.GlobalUser{
				ID:             1,
				PrimaryChannel: model.ChannelTelegram,
				Accounts: []model.ChannelAccount{
					{ChannelType: model.ChannelTelegram, ChannelUserID: "tg-1"},
					{ChannelType: model.ChannelDiscord, ChannelUserID: "dc-1"},
				},
			}, nil
		},
	}

	prefs := &mockPrefsRepo{
		getPrefsFn: func(_ context.Context, _ model.GlobalUserID) (*model.NotificationPrefs, error) {
			return &model.NotificationPrefs{
				GlobalUserID:    1,
				ChannelPriority: []model.ChannelType{model.ChannelDiscord},
			}, nil
		},
	}

	reg := newRegistryWithAdapters(adapter, &mockAdapter{channelType: model.ChannelTelegram})
	api := NewNotifyAPI(reg, users, prefs, nil)

	err := api.NotifyUser(context.Background(), 1, model.NewTextMessage("test"), model.PriorityNormal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedPlatformID != "dc-1" {
		t.Errorf("sent to platform ID = %s, want dc-1", capturedPlatformID)
	}
}

func TestNotifyUser_DefaultPrefsWhenNil(t *testing.T) {
	t.Parallel()

	var capturedPlatformID model.PlatformUserID

	adapter := &mockAdapter{
		channelType: model.ChannelTelegram,
		sendToUserFn: func(_ context.Context, pid model.PlatformUserID, _ model.Message) error {
			capturedPlatformID = pid
			return nil
		},
	}

	users := &mockUserService{
		getUserFn: func(_ context.Context, _ model.GlobalUserID) (*model.GlobalUser, error) {
			return &model.GlobalUser{
				ID:             1,
				PrimaryChannel: model.ChannelTelegram,
				Accounts: []model.ChannelAccount{
					{ChannelType: model.ChannelTelegram, ChannelUserID: "tg-1"},
				},
			}, nil
		},
	}

	prefs := &mockPrefsRepo{
		getPrefsFn: func(_ context.Context, _ model.GlobalUserID) (*model.NotificationPrefs, error) {
			return nil, nil // no prefs stored
		},
	}

	reg := newRegistryWithAdapters(adapter)
	api := NewNotifyAPI(reg, users, prefs, nil)

	err := api.NotifyUser(context.Background(), 1, model.NewTextMessage("test"), model.PriorityNormal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedPlatformID != "tg-1" {
		t.Errorf("sent to platform ID = %s, want tg-1 (primary channel default)", capturedPlatformID)
	}
}
