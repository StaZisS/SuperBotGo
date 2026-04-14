package mattermost

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/model"

	mm "github.com/mattermost/mattermost/server/public/model"
)

func TestHandleActionDispatchesCallback(t *testing.T) {
	updates := make(chan channel.Update, 1)
	bot := &Bot{
		actionsSecret: "secret",
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		handler: func(ctx context.Context, update channel.Update) error {
			updates <- update
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/mattermost/actions", actionPayload(t, mm.PostActionIntegrationRequest{
		UserId:    "user-1",
		UserName:  "alice",
		ChannelId: "channel-1",
		PostId:    "post-1",
		TriggerId: "trigger-1",
		Context: map[string]any{
			actionContextSecretKey: "secret",
			actionContextValueKey:  "/plugins",
			actionContextLabelKey:  "Plugins",
		},
	}))
	rec := httptest.NewRecorder()

	bot.handleAction(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	update := waitForUpdate(t, updates)
	callback, ok := update.Input.(model.CallbackInput)
	if !ok {
		t.Fatalf("input type = %T, want model.CallbackInput", update.Input)
	}
	if callback.Data != "/plugins" {
		t.Fatalf("callback data = %q, want /plugins", callback.Data)
	}
	if callback.Label != "Plugins" {
		t.Fatalf("callback label = %q, want Plugins", callback.Label)
	}
	if update.PlatformUpdateID != "mm:action:post-1:trigger-1" {
		t.Fatalf("update id = %q, want mm:action:post-1:trigger-1", update.PlatformUpdateID)
	}
}

func TestHandleActionDefaultsEmptyLabelToValue(t *testing.T) {
	updates := make(chan channel.Update, 1)
	bot := &Bot{
		actionsSecret: "secret",
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		handler: func(ctx context.Context, update channel.Update) error {
			updates <- update
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/mattermost/actions", actionPayload(t, mm.PostActionIntegrationRequest{
		UserId:    "user-1",
		ChannelId: "channel-1",
		PostId:    "post-1",
		Context: map[string]any{
			actionContextSecretKey: "secret",
			actionContextValueKey:  "next",
		},
	}))
	rec := httptest.NewRecorder()

	bot.handleAction(rec, req)

	update := waitForUpdate(t, updates)
	callback, ok := update.Input.(model.CallbackInput)
	if !ok {
		t.Fatalf("input type = %T, want model.CallbackInput", update.Input)
	}
	if callback.Label != "next" {
		t.Fatalf("callback label = %q, want next", callback.Label)
	}
}

func TestHandleActionRejectsInvalidSecret(t *testing.T) {
	called := make(chan struct{}, 1)
	bot := &Bot{
		actionsSecret: "secret",
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		handler: func(ctx context.Context, update channel.Update) error {
			called <- struct{}{}
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/mattermost/actions", actionPayload(t, mm.PostActionIntegrationRequest{
		UserId:    "user-1",
		ChannelId: "channel-1",
		PostId:    "post-1",
		Context: map[string]any{
			actionContextSecretKey: "wrong",
			actionContextValueKey:  "next",
		},
	}))
	rec := httptest.NewRecorder()

	bot.handleAction(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	select {
	case <-called:
		t.Fatal("handler called, want not called")
	case <-time.After(100 * time.Millisecond):
	}
}

func actionPayload(t *testing.T, req mm.PostActionIntegrationRequest) *bytes.Reader {
	t.Helper()

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal action request: %v", err)
	}
	return bytes.NewReader(body)
}

func waitForUpdate(t *testing.T, updates <-chan channel.Update) channel.Update {
	t.Helper()

	select {
	case update := <-updates:
		return update
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for update")
		return channel.Update{}
	}
}
