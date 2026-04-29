package vk

import (
	"reflect"
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
	wantText := strings.Join([]string{
		"Header",
		"Subheader",
		"> line 1",
		"> line 2",
		"echo hi",
	}, "\n")

	if rendered.Text != wantText {
		t.Fatalf("Render().Text = %q, want %q", rendered.Text, wantText)
	}

	assertFormatData(t, rendered.FormatData, []vkFormatItem{
		{Type: "bold", Offset: 0, Length: utf16Len("Header")},
		{Type: "italic", Offset: utf16Len("Header\n"), Length: utf16Len("Subheader")},
	})
}

func TestVKRenderer_RenderLinkAndPlainSymbols(t *testing.T) {
	renderer := NewRenderer()
	msg := model.Message{
		Blocks: []model.ContentBlock{
			model.TextBlock{Text: "2 < 3 & 5", Style: model.StylePlain},
			model.TextBlock{Text: "bold <tag>", Style: model.StyleHeader},
			model.LinkBlock{URL: "https://example.com?q=1&x=2", Label: "Open <site>"},
		},
	}

	rendered := renderer.Render(msg)
	wantText := strings.Join([]string{
		"2 < 3 & 5",
		"bold <tag>",
		"Open <site>",
	}, "\n")

	if rendered.Text != wantText {
		t.Fatalf("Render().Text = %q, want %q", rendered.Text, wantText)
	}

	assertFormatData(t, rendered.FormatData, []vkFormatItem{
		{
			Type:   "bold",
			Offset: utf16Len("2 < 3 & 5\n"),
			Length: utf16Len("bold <tag>"),
		},
		{
			Type:   "url",
			Offset: utf16Len("2 < 3 & 5\nbold <tag>\n"),
			Length: utf16Len("Open <site>"),
			URL:    "https://example.com?q=1&x=2",
		},
	})
}

func TestVKRenderer_UsesUTF16Offsets(t *testing.T) {
	renderer := NewRenderer()
	msg := model.Message{
		Blocks: []model.ContentBlock{
			model.TextBlock{Text: "🙂", Style: model.StylePlain},
			model.TextBlock{Text: "Жир", Style: model.StyleHeader},
		},
	}

	rendered := renderer.Render(msg)
	assertFormatData(t, rendered.FormatData, []vkFormatItem{
		{Type: "bold", Offset: 3, Length: utf16Len("Жир")},
	})
}

func TestVKRenderer_RenderTruncatesToVKLimit(t *testing.T) {
	renderer := NewRenderer()
	msg := model.NewStyledTextMessage(strings.Repeat("а", vkMaxMessageLength+10), model.StyleHeader)

	rendered := renderer.Render(msg)
	if got := runeCount(rendered.Text); got != vkMaxMessageLength {
		t.Fatalf("Render().Text length = %d, want %d", got, vkMaxMessageLength)
	}
	if !strings.HasSuffix(rendered.Text, "...") {
		t.Fatalf("Render().Text = %q, want suffix %q", rendered.Text[len(rendered.Text)-6:], "...")
	}

	assertFormatData(t, rendered.FormatData, []vkFormatItem{
		{Type: "bold", Offset: 0, Length: vkMaxMessageLength - 3},
	})
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

func assertFormatData(t *testing.T, got *vkFormatData, want []vkFormatItem) {
	t.Helper()

	if got == nil {
		t.Fatal("expected format data, got nil")
	}
	if got.Version != 1 {
		t.Fatalf("format data version = %d, want 1", got.Version)
	}
	if !reflect.DeepEqual(got.Items, want) {
		t.Fatalf("format data items = %#v, want %#v", got.Items, want)
	}
}
