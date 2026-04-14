package vk

import (
	"strings"
	"testing"

	"SuperBotGo/internal/model"
)

func TestVKRenderer_RenderStyledText(t *testing.T) {
	renderer := NewRenderer()
	msg := model.Message{
		Blocks: []model.ContentBlock{
			model.TextBlock{Text: "Header", Style: model.StyleHeader},
			model.TextBlock{Text: "Subheader", Style: model.StyleSubheader},
			model.TextBlock{Text: "line 1\nline 2", Style: model.StyleQuote},
			model.TextBlock{Text: "echo hi", Style: model.StyleCode},
		},
	}

	rendered := renderer.Render(msg)
	want := strings.Join([]string{
		"*Header*",
		"_Subheader_",
		"> line 1",
		"> line 2",
		"`echo hi`",
	}, "\n")

	if rendered.Text != want {
		t.Fatalf("Render().Text = %q, want %q", rendered.Text, want)
	}
}

func TestVKRenderer_RenderMultilineCode(t *testing.T) {
	renderer := NewRenderer()
	msg := model.NewStyledTextMessage("first()\nsecond()", model.StyleCode)

	rendered := renderer.Render(msg)
	want := "```\nfirst()\nsecond()\n```"
	if rendered.Text != want {
		t.Fatalf("Render().Text = %q, want %q", rendered.Text, want)
	}
}

func TestVKRenderer_RenderTruncatesToVKLimit(t *testing.T) {
	renderer := NewRenderer()
	msg := model.NewTextMessage(strings.Repeat("а", vkMaxMessageLength+10))

	rendered := renderer.Render(msg)
	if got := runeCount(rendered.Text); got != vkMaxMessageLength {
		t.Fatalf("Render().Text length = %d, want %d", got, vkMaxMessageLength)
	}
	if !strings.HasSuffix(rendered.Text, "...") {
		t.Fatalf("Render().Text = %q, want suffix %q", rendered.Text[len(rendered.Text)-6:], "...")
	}
}

func TestVKButtonLabel_StripsDescriptionWhenLabelTooLong(t *testing.T) {
	opt := model.Option{
		Label: "/resume — Resume active command on this platform",
		Value: "/core.resume",
	}

	got := vkButtonLabel(opt)
	want := "/resume"
	if got != want {
		t.Fatalf("vkButtonLabel() = %q, want %q", got, want)
	}
}

func TestVKButtonLabel_FallsBackToValueWhenValueIsShorter(t *testing.T) {
	opt := model.Option{
		Label: "This is a very long button label without a separator",
		Value: "/plugins",
	}

	got := vkButtonLabel(opt)
	want := "/plugins"
	if got != want {
		t.Fatalf("vkButtonLabel() = %q, want %q", got, want)
	}
}

func TestVKButtonLabel_TruncatesAsLastResort(t *testing.T) {
	opt := model.Option{
		Label: "123456789012345678901234567890123456789012345",
		Value: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
	}

	got := vkButtonLabel(opt)
	if runeCount(got) != vkMaxButtonLabel {
		t.Fatalf("vkButtonLabel() length = %d, want %d", runeCount(got), vkMaxButtonLabel)
	}
}

func TestNormalizeVKOptions_CommandLabelsStayInTextButButtonsAreShort(t *testing.T) {
	options := &model.OptionsBlock{
		Prompt: "Choose command:",
		Options: []model.Option{
			{Label: "/link — Account Linking", Value: "/core.link"},
			{Label: "/settings — User Settings", Value: "/core.settings"},
		},
	}

	normalized, extraText := normalizeVKOptions(options)
	if normalized == nil {
		t.Fatal("expected normalized options")
	}
	if got := normalized.Options[0].Label; got != "/link" {
		t.Fatalf("first button label = %q, want %q", got, "/link")
	}
	if got := normalized.Options[1].Label; got != "/settings" {
		t.Fatalf("second button label = %q, want %q", got, "/settings")
	}
	if len(extraText) != 2 {
		t.Fatalf("extraText len = %d, want 2", len(extraText))
	}
	if extraText[0] != "/link — Account Linking" {
		t.Fatalf("extraText[0] = %q", extraText[0])
	}
}
