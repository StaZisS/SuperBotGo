package vk

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"SuperBotGo/internal/model"

	vkapi "github.com/SevereCloud/vksdk/v3/api"
)

func TestAdapterSendToUser_SendsFormatData(t *testing.T) {
	var gotForm url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages.send" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if err := r.Body.Close(); err != nil {
			t.Fatalf("close request body: %v", err)
		}

		gotForm, err = url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse form body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":1}`))
	}))
	defer server.Close()

	vkClient := vkapi.NewVK("group-token")
	vkClient.Client = server.Client()
	vkClient.MethodURL = server.URL + "/"

	connected := &atomic.Bool{}
	connected.Store(true)

	adapter := NewAdapter(vkClient, connected, nil)
	msg := model.Message{
		Blocks: []model.ContentBlock{
			model.TextBlock{Text: "Плагины", Style: model.StyleHeader},
			model.TextBlock{Text: "Выберите плагин, чтобы увидеть его команды:", Style: model.StylePlain},
		},
	}

	if err := adapter.SendToUser(context.Background(), "123", msg); err != nil {
		t.Fatalf("SendToUser() error = %v", err)
	}

	wantText := "Плагины\nВыберите плагин, чтобы увидеть его команды:"
	if got := gotForm.Get("message"); got != wantText {
		t.Fatalf("message = %q, want %q", got, wantText)
	}

	var formatData vkFormatData
	if err := json.Unmarshal([]byte(gotForm.Get("format_data")), &formatData); err != nil {
		t.Fatalf("unmarshal format_data: %v", err)
	}

	if formatData.Version != 1 {
		t.Fatalf("format_data.version = %d, want 1", formatData.Version)
	}
	if len(formatData.Items) != 1 {
		t.Fatalf("format_data.items len = %d, want 1", len(formatData.Items))
	}

	item := formatData.Items[0]
	if item.Type != "bold" {
		t.Fatalf("format_data.items[0].type = %q, want %q", item.Type, "bold")
	}
	if item.Offset != 0 {
		t.Fatalf("format_data.items[0].offset = %d, want 0", item.Offset)
	}
	if item.Length != utf16Len("Плагины") {
		t.Fatalf("format_data.items[0].length = %d, want %d", item.Length, utf16Len("Плагины"))
	}
}
