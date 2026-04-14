package mattermost

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/model"
)

func TestHandleCommandDispatchesSlashCommand(t *testing.T) {
	updates := make(chan channel.Update, 1)
	bot := &Bot{
		commandTrigger: "hits",
		commandToken:   "secret",
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		handler: func(ctx context.Context, update channel.Update) error {
			updates <- update
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/mattermost/command", strings.NewReader(validCommandForm(url.Values{
		"team_id":    []string{"team-1"},
		"channel_id": []string{"channel-1"},
		"user_id":    []string{"user-1"},
		"user_name":  []string{"alice"},
		"command":    []string{"/hits"},
		"text":       []string{"plugins"},
		"token":      []string{"secret"},
		"trigger_id": []string{"trigger-1"},
	})))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	bot.handleCommand(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), `"response_type":"ephemeral"`) {
		t.Fatalf("response body = %q, want ephemeral ack", rec.Body.String())
	}

	update := waitForUpdate(t, updates)
	text, ok := update.Input.(model.TextInput)
	if !ok {
		t.Fatalf("input type = %T, want model.TextInput", update.Input)
	}
	if text.Text != "/plugins" {
		t.Fatalf("text = %q, want /plugins", text.Text)
	}
	if update.ChatID != "channel-1" {
		t.Fatalf("chat id = %q, want channel-1", update.ChatID)
	}
	if update.Username != "alice" {
		t.Fatalf("username = %q, want alice", update.Username)
	}
}

func TestHandleCommandDefaultsEmptyTextToStart(t *testing.T) {
	updates := make(chan channel.Update, 1)
	bot := &Bot{
		commandTrigger: "hits",
		commandToken:   "secret",
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		handler: func(ctx context.Context, update channel.Update) error {
			updates <- update
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/mattermost/command", strings.NewReader(validCommandForm(url.Values{
		"team_id":    []string{"team-1"},
		"channel_id": []string{"channel-1"},
		"user_id":    []string{"user-1"},
		"command":    []string{"/hits"},
		"text":       []string{""},
		"token":      []string{"secret"},
		"trigger_id": []string{"trigger-1"},
	})))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	bot.handleCommand(rec, req)

	update := waitForUpdate(t, updates)
	text, ok := update.Input.(model.TextInput)
	if !ok {
		t.Fatalf("input type = %T, want model.TextInput", update.Input)
	}
	if text.Text != "/start" {
		t.Fatalf("text = %q, want /start", text.Text)
	}
}

func TestHandleCommandRejectsInvalidToken(t *testing.T) {
	called := make(chan struct{}, 1)
	bot := &Bot{
		commandTrigger: "hits",
		commandToken:   "secret",
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		handler: func(ctx context.Context, update channel.Update) error {
			called <- struct{}{}
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/mattermost/command", strings.NewReader(validCommandForm(url.Values{
		"team_id":    []string{"team-1"},
		"channel_id": []string{"channel-1"},
		"user_id":    []string{"user-1"},
		"command":    []string{"/hits"},
		"text":       []string{"plugins"},
		"token":      []string{"wrong"},
		"trigger_id": []string{"trigger-1"},
	})))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	bot.handleCommand(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	select {
	case <-called:
		t.Fatal("handler called, want not called")
	case <-time.After(100 * time.Millisecond):
	}
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

func validCommandForm(values url.Values) string {
	if values.Get("team_domain") == "" {
		values.Set("team_domain", "test-team")
	}
	if values.Get("channel_name") == "" {
		values.Set("channel_name", "town-square")
	}
	return values.Encode()
}
