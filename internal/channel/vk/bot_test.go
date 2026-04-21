package vk

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/filestore"
	"SuperBotGo/internal/model"

	"github.com/SevereCloud/vksdk/v3/events"
	vkobject "github.com/SevereCloud/vksdk/v3/object"
)

func TestIsVKStartButtonMessage(t *testing.T) {
	msg := vkobject.MessagesMessage{
		PeerID:                123,
		FromID:                123,
		ConversationMessageID: 1,
		Text:                  "Начать",
	}

	if !isVKStartButtonMessage(msg) {
		t.Fatal("expected start button message to be detected")
	}
}

func TestIsVKStartButtonMessage_IgnoresLaterMessages(t *testing.T) {
	msg := vkobject.MessagesMessage{
		PeerID:                123,
		FromID:                123,
		ConversationMessageID: 2,
		Text:                  "Начать",
	}

	if isVKStartButtonMessage(msg) {
		t.Fatal("expected non-initial message to be ignored")
	}
}

func TestBuildInput_MapsVKStartButtonToSlashStart(t *testing.T) {
	bot := &Bot{}
	msg := vkobject.MessagesMessage{
		PeerID:                123,
		FromID:                123,
		ConversationMessageID: 1,
		Text:                  "Начать",
	}

	input, ok := bot.buildInput(t.Context(), msg)
	if !ok {
		t.Fatal("expected input to be built")
	}

	text, ok := input.(model.TextInput)
	if !ok {
		t.Fatalf("expected TextInput, got %T", input)
	}
	if text.Text != "/start" {
		t.Fatalf("expected /start, got %q", text.Text)
	}
}

func TestHandleMessageNew_UsesLifecycleContextForFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("payload"))
	}))
	defer server.Close()

	store := &capturingFileStore{}

	var (
		gotCtx    context.Context
		gotUpdate channel.Update
	)

	bot := &Bot{
		fileStore:    store,
		httpClient:   server.Client(),
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		lifecycleCtx: context.Background(),
		handler: func(ctx context.Context, update channel.Update) error {
			gotCtx = ctx
			gotUpdate = update
			return nil
		},
	}

	eventCtx, cancel := context.WithCancel(context.Background())
	cancel()

	bot.handleMessageNew(eventCtx, events.MessageNewObject{
		Message: vkobject.MessagesMessage{
			PeerID: 101,
			FromID: 202,
			ID:     303,
			Text:   "caption",
			Attachments: []vkobject.MessagesMessageAttachment{
				{
					Type: "doc",
					Doc: vkobject.DocsDoc{
						URL:   server.URL,
						Title: "test.txt",
						Size:  7,
					},
				},
			},
		},
	})

	if store.storeCtx == nil {
		t.Fatal("expected file store to receive context")
	}
	if err := store.storeCtx.Err(); err != nil {
		t.Fatalf("expected lifecycle context in file store, got %v", err)
	}
	if gotCtx == nil {
		t.Fatal("expected handler to be called")
	}
	if err := gotCtx.Err(); err != nil {
		t.Fatalf("expected lifecycle context in handler, got %v", err)
	}

	input, ok := gotUpdate.Input.(model.FileInput)
	if !ok {
		t.Fatalf("expected FileInput, got %T", gotUpdate.Input)
	}
	if len(input.Files) != 1 || input.Files[0].ID != "stored-file" {
		t.Fatalf("unexpected files: %#v", input.Files)
	}
}

type capturingFileStore struct {
	storeCtx context.Context
}

func (s *capturingFileStore) Store(ctx context.Context, meta filestore.FileMeta, data io.Reader) (model.FileRef, error) {
	s.storeCtx = ctx
	payload, err := io.ReadAll(data)
	if err != nil {
		return model.FileRef{}, err
	}
	return model.FileRef{
		ID:       "stored-file",
		Name:     meta.Name,
		MIMEType: meta.MIMEType,
		Size:     int64(len(payload)),
		FileType: meta.FileType,
	}, nil
}

func (s *capturingFileStore) Get(context.Context, string) (io.ReadCloser, *filestore.FileMeta, error) {
	return nil, nil, nil
}

func (s *capturingFileStore) Meta(context.Context, string) (*filestore.FileMeta, error) {
	return nil, nil
}

func (s *capturingFileStore) Delete(context.Context, string) error {
	return nil
}

func (s *capturingFileStore) URL(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

func (s *capturingFileStore) Cleanup(context.Context) (int, error) {
	return 0, nil
}
