package vk

import (
	"testing"

	"SuperBotGo/internal/model"

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
