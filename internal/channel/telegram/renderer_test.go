package telegram

import (
	"testing"

	"SuperBotGo/internal/model"
)

func TestRenderOptionsWithoutPromptProducesTextFallback(t *testing.T) {
	renderer := NewRenderer()

	msg := model.NewOptionsMessage("", []model.Option{
		{Label: "First", Value: "one"},
		{Value: "two"},
	})

	rendered := renderer.Render(msg)
	if rendered.Text == "" {
		t.Fatal("Render() produced empty text for options-only message")
	}
	if len(rendered.Keyboard) != 2 {
		t.Fatalf("Render() keyboard rows = %d, want 2", len(rendered.Keyboard))
	}
}
