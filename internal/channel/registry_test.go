package channel

import (
	"context"
	"errors"
	"strings"
	"testing"

	"SuperBotGo/internal/model"
)

// ---------------------------------------------------------------------------
// Mock adapter
// ---------------------------------------------------------------------------

type mockAdapter struct {
	channelType  model.ChannelType
	sendToUserFn func(ctx context.Context, uid model.PlatformUserID, msg model.Message) error
	sendToChatFn func(ctx context.Context, chatID string, msg model.Message) error
}

func newMockAdapter(ct model.ChannelType) *mockAdapter {
	return &mockAdapter{
		channelType: ct,
		sendToUserFn: func(context.Context, model.PlatformUserID, model.Message) error {
			return nil
		},
		sendToChatFn: func(context.Context, string, model.Message) error {
			return nil
		},
	}
}

func (m *mockAdapter) Type() model.ChannelType { return m.channelType }

func (m *mockAdapter) SendToUser(ctx context.Context, platformUserID model.PlatformUserID, msg model.Message) error {
	return m.sendToUserFn(ctx, platformUserID, msg)
}

func (m *mockAdapter) SendToChat(ctx context.Context, chatID string, msg model.Message) error {
	return m.sendToChatFn(ctx, chatID, msg)
}

var _ ChannelAdapter = (*mockAdapter)(nil)

// ---------------------------------------------------------------------------
// NewAdapterRegistry
// ---------------------------------------------------------------------------

func TestNewAdapterRegistry(t *testing.T) {
	r := NewAdapterRegistry()
	if r == nil {
		t.Fatal("NewAdapterRegistry returned nil")
	}
}

// ---------------------------------------------------------------------------
// Register + Get
// ---------------------------------------------------------------------------

func TestRegisterAndGet(t *testing.T) {
	tests := []struct {
		name       string
		registerCT model.ChannelType
		getCT      model.ChannelType
		wantNil    bool
	}{
		{
			name:       "get registered adapter",
			registerCT: model.ChannelTelegram,
			getCT:      model.ChannelTelegram,
			wantNil:    false,
		},
		{
			name:       "get unregistered adapter returns nil",
			registerCT: model.ChannelTelegram,
			getCT:      model.ChannelDiscord,
			wantNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewAdapterRegistry()
			adapter := newMockAdapter(tt.registerCT)
			r.Register(adapter)

			got := r.Get(tt.getCT)
			if tt.wantNil && got != nil {
				t.Errorf("Get(%q) = %v, want nil", tt.getCT, got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("Get(%q) = nil, want adapter", tt.getCT)
			}
			if !tt.wantNil && got != nil && got.Type() != tt.registerCT {
				t.Errorf("Get(%q).Type() = %q, want %q", tt.getCT, got.Type(), tt.registerCT)
			}
		})
	}
}

func TestGet_EmptyRegistry(t *testing.T) {
	r := NewAdapterRegistry()
	got := r.Get(model.ChannelTelegram)
	if got != nil {
		t.Errorf("Get on empty registry = %v, want nil", got)
	}
}

// ---------------------------------------------------------------------------
// IsRegistered
// ---------------------------------------------------------------------------

func TestIsRegistered(t *testing.T) {
	tests := []struct {
		name     string
		register []model.ChannelType
		query    model.ChannelType
		want     bool
	}{
		{
			name:     "registered type returns true",
			register: []model.ChannelType{model.ChannelTelegram},
			query:    model.ChannelTelegram,
			want:     true,
		},
		{
			name:     "unregistered type returns false",
			register: []model.ChannelType{model.ChannelTelegram},
			query:    model.ChannelDiscord,
			want:     false,
		},
		{
			name:     "empty registry returns false",
			register: nil,
			query:    model.ChannelTelegram,
			want:     false,
		},
		{
			name:     "multiple registered - query first",
			register: []model.ChannelType{model.ChannelTelegram, model.ChannelDiscord},
			query:    model.ChannelTelegram,
			want:     true,
		},
		{
			name:     "multiple registered - query second",
			register: []model.ChannelType{model.ChannelTelegram, model.ChannelDiscord},
			query:    model.ChannelDiscord,
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewAdapterRegistry()
			for _, ct := range tt.register {
				r.Register(newMockAdapter(ct))
			}

			got := r.IsRegistered(tt.query)
			if got != tt.want {
				t.Errorf("IsRegistered(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Register overwrites previous adapter
// ---------------------------------------------------------------------------

func TestRegister_Overwrites(t *testing.T) {
	r := NewAdapterRegistry()
	first := newMockAdapter(model.ChannelTelegram)
	second := newMockAdapter(model.ChannelTelegram)

	r.Register(first)
	r.Register(second)

	got := r.Get(model.ChannelTelegram)
	if got != second {
		t.Error("Register did not overwrite the previous adapter")
	}
}

// ---------------------------------------------------------------------------
// mustGet
// ---------------------------------------------------------------------------

func TestMustGet(t *testing.T) {
	tests := []struct {
		name      string
		register  []model.ChannelType
		query     model.ChannelType
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "registered type succeeds",
			register: []model.ChannelType{model.ChannelTelegram},
			query:    model.ChannelTelegram,
			wantErr:  false,
		},
		{
			name:      "unregistered type returns error",
			register:  nil,
			query:     model.ChannelDiscord,
			wantErr:   true,
			errSubstr: "no adapter registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewAdapterRegistry()
			for _, ct := range tt.register {
				r.Register(newMockAdapter(ct))
			}

			adapter, err := r.mustGet(tt.query)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				if adapter != nil {
					t.Error("expected nil adapter on error")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if adapter == nil {
					t.Fatal("expected adapter, got nil")
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SendToChat
// ---------------------------------------------------------------------------

func TestSendToChat(t *testing.T) {
	t.Run("registered adapter - delegates call", func(t *testing.T) {
		r := NewAdapterRegistry()
		var capturedChatID string
		var capturedMsg model.Message
		adapter := newMockAdapter(model.ChannelTelegram)
		adapter.sendToChatFn = func(_ context.Context, chatID string, msg model.Message) error {
			capturedChatID = chatID
			capturedMsg = msg
			return nil
		}
		r.Register(adapter)

		msg := model.NewTextMessage("hello")
		err := r.SendToChat(context.Background(), model.ChannelTelegram, "chat-123", msg)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capturedChatID != "chat-123" {
			t.Errorf("chatID = %q, want %q", capturedChatID, "chat-123")
		}
		if len(capturedMsg.Blocks) != 1 {
			t.Errorf("msg has %d blocks, want 1", len(capturedMsg.Blocks))
		} else if tb, ok := capturedMsg.Blocks[0].(model.TextBlock); !ok || tb.Text != "hello" {
			t.Errorf("msg block = %+v, want TextBlock with 'hello'", capturedMsg.Blocks[0])
		}
	})

	t.Run("unregistered adapter - returns error", func(t *testing.T) {
		r := NewAdapterRegistry()
		err := r.SendToChat(context.Background(), model.ChannelDiscord, "chat-456", model.Message{})

		if err == nil {
			t.Fatal("expected error for unregistered adapter, got nil")
		}
		if !strings.Contains(err.Error(), "no adapter registered") {
			t.Errorf("error %q does not indicate missing adapter", err.Error())
		}
	})

	t.Run("adapter returns error", func(t *testing.T) {
		r := NewAdapterRegistry()
		adapterErr := errors.New("invalid input")
		adapter := newMockAdapter(model.ChannelTelegram)
		adapter.sendToChatFn = func(context.Context, string, model.Message) error {
			return adapterErr
		}
		r.Register(adapter)

		err := r.SendToChat(context.Background(), model.ChannelTelegram, "chat-123", model.Message{})

		if !errors.Is(err, adapterErr) {
			t.Errorf("expected %v, got %v", adapterErr, err)
		}
	})
}

// ---------------------------------------------------------------------------
// SendToUser
// ---------------------------------------------------------------------------

func TestSendToUser(t *testing.T) {
	t.Run("registered adapter - delegates call", func(t *testing.T) {
		r := NewAdapterRegistry()
		var capturedUID model.PlatformUserID
		var capturedMsg model.Message
		adapter := newMockAdapter(model.ChannelTelegram)
		adapter.sendToUserFn = func(_ context.Context, uid model.PlatformUserID, msg model.Message) error {
			capturedUID = uid
			capturedMsg = msg
			return nil
		}
		r.Register(adapter)

		msg := model.NewTextMessage("hi user")
		err := r.SendToUser(context.Background(), model.ChannelTelegram, "user-42", msg)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capturedUID != "user-42" {
			t.Errorf("platformUserID = %q, want %q", capturedUID, "user-42")
		}
		if len(capturedMsg.Blocks) != 1 {
			t.Errorf("msg has %d blocks, want 1", len(capturedMsg.Blocks))
		} else if tb, ok := capturedMsg.Blocks[0].(model.TextBlock); !ok || tb.Text != "hi user" {
			t.Errorf("msg block = %+v, want TextBlock with 'hi user'", capturedMsg.Blocks[0])
		}
	})

	t.Run("unregistered adapter - returns error", func(t *testing.T) {
		r := NewAdapterRegistry()
		err := r.SendToUser(context.Background(), model.ChannelDiscord, "user-1", model.Message{})

		if err == nil {
			t.Fatal("expected error for unregistered adapter, got nil")
		}
		if !strings.Contains(err.Error(), "no adapter registered") {
			t.Errorf("error %q does not indicate missing adapter", err.Error())
		}
	})

	t.Run("adapter returns non-transient error", func(t *testing.T) {
		r := NewAdapterRegistry()
		adapterErr := errors.New("user not found")
		adapter := newMockAdapter(model.ChannelTelegram)
		adapter.sendToUserFn = func(context.Context, model.PlatformUserID, model.Message) error {
			return adapterErr
		}
		r.Register(adapter)

		err := r.SendToUser(context.Background(), model.ChannelTelegram, "user-42", model.Message{})

		if !errors.Is(err, adapterErr) {
			t.Errorf("expected %v, got %v", adapterErr, err)
		}
	})
}

// ---------------------------------------------------------------------------
// Concurrent access
// ---------------------------------------------------------------------------

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := NewAdapterRegistry()
	r.Register(newMockAdapter(model.ChannelTelegram))

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			r.Register(newMockAdapter(model.ChannelDiscord))
			r.IsRegistered(model.ChannelTelegram)
			r.Get(model.ChannelDiscord)
		}
	}()

	for i := 0; i < 100; i++ {
		r.Get(model.ChannelTelegram)
		r.IsRegistered(model.ChannelDiscord)
		r.Register(newMockAdapter(model.ChannelTelegram))
	}

	<-done
}
