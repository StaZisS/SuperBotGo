package vk

import (
	"fmt"
	"strings"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/model"

	vkobject "github.com/SevereCloud/vksdk/v3/object"
)

const (
	vkMaxMessageLength = 4096
	vkMaxButtonLabel   = 40
	vkPayloadKey       = "sb"
	maxKeyboardButtons = 10
)

type buttonPayload struct {
	Value string `json:"sb"`
}

type RenderedMessage struct {
	Text      string
	Keyboard  *vkobject.MessagesKeyboard
	ImageURLs []string
	FileRefs  []model.FileRef
}

type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

type formatter struct{}

func (formatter) FormatText(b model.TextBlock) string {
	switch b.Style {
	case model.StyleHeader:
		return strings.ToUpper(b.Text)
	case model.StyleSubheader:
		return fmt.Sprintf("• %s", b.Text)
	case model.StyleCode:
		return fmt.Sprintf("`%s`", b.Text)
	case model.StyleQuote:
		return fmt.Sprintf("> %s", b.Text)
	default:
		return b.Text
	}
}

func (formatter) FormatMention(b model.MentionBlock) string {
	username := b.Username
	if username == "" {
		username = "user"
	}
	return fmt.Sprintf("[id%s|%s]", b.UserID, username)
}

func (formatter) FormatLink(b model.LinkBlock) string {
	if b.Label == "" || b.Label == b.URL {
		return b.URL
	}
	return fmt.Sprintf("%s: %s", b.Label, b.URL)
}

func (formatter) FormatOptionsPrompt(prompt string) string {
	return prompt
}

func (r *Renderer) Render(msg model.Message) RenderedMessage {
	parts := channel.RenderBlocks(msg, formatter{})
	options, extraText := normalizeVKOptions(parts.Options)

	textParts := make([]string, 0, len(parts.TextParts)+len(extraText))
	textParts = append(textParts, parts.TextParts...)
	textParts = append(textParts, extraText...)

	text := strings.Join(textParts, "\n")
	if len([]rune(text)) > vkMaxMessageLength {
		runes := []rune(text)
		text = string(runes[:vkMaxMessageLength-3]) + "..."
	}

	return RenderedMessage{
		Text:      text,
		Keyboard:  buildKeyboard(options),
		ImageURLs: parts.ImageURLs,
		FileRefs:  parts.FileRefs,
	}
}

func buildKeyboard(options *model.OptionsBlock) *vkobject.MessagesKeyboard {
	if options == nil || len(options.Options) == 0 {
		return nil
	}

	keyboard := vkobject.NewMessagesKeyboard(true)
	limit := len(options.Options)
	if limit > maxKeyboardButtons {
		limit = maxKeyboardButtons
	}

	for _, opt := range options.Options[:limit] {
		label := vkButtonLabel(opt)
		if label == "" {
			continue
		}
		keyboard.AddRow()
		keyboard.AddTextButton(label, buttonPayload{Value: opt.Value}, vkobject.Primary)
	}

	return keyboard
}

func normalizeVKOptions(options *model.OptionsBlock) (*model.OptionsBlock, []string) {
	if options == nil || len(options.Options) == 0 {
		return options, nil
	}

	normalized := *options
	normalized.Options = make([]model.Option, 0, len(options.Options))
	extraText := make([]string, 0, len(options.Options))

	for _, opt := range options.Options {
		normalizedOpt := opt
		label := channel.OptionLabel(opt)

		if commandLabel, ok := shortVKCommandLabel(label); ok {
			normalizedOpt.Label = commandLabel
			extraText = append(extraText, label)
		} else {
			normalizedOpt.Label = vkButtonLabel(opt)
		}

		normalized.Options = append(normalized.Options, normalizedOpt)
	}

	return &normalized, extraText
}

func shortVKCommandLabel(label string) (string, bool) {
	if !strings.HasPrefix(label, "/") {
		return "", false
	}

	head, _, ok := strings.Cut(label, " — ")
	if !ok {
		return "", false
	}
	head = strings.TrimSpace(head)
	if head == "" {
		return "", false
	}
	if runeCount(head) > vkMaxButtonLabel {
		return "", false
	}

	return head, true
}

func vkButtonLabel(opt model.Option) string {
	label := channel.OptionLabel(opt)
	if label == "" {
		return ""
	}

	if runeCount(label) <= vkMaxButtonLabel {
		return label
	}

	// Prefer the concise command/button title without the description suffix.
	if head, _, ok := strings.Cut(label, " — "); ok && runeCount(head) <= vkMaxButtonLabel {
		return head
	}

	if runeCount(opt.Value) > 0 && runeCount(opt.Value) <= vkMaxButtonLabel {
		return opt.Value
	}

	return truncateRunes(label, vkMaxButtonLabel)
}

func truncateRunes(s string, limit int) string {
	if limit <= 0 {
		return ""
	}

	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	if limit == 1 {
		return "…"
	}

	return string(runes[:limit-1]) + "…"
}

func runeCount(s string) int {
	return len([]rune(s))
}
