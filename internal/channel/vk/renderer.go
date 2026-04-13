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

	text := strings.Join(parts.TextParts, "\n")
	if len([]rune(text)) > vkMaxMessageLength {
		runes := []rune(text)
		text = string(runes[:vkMaxMessageLength-3]) + "..."
	}

	return RenderedMessage{
		Text:      text,
		Keyboard:  buildKeyboard(parts.Options),
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
		label := channel.OptionLabel(opt)
		if label == "" {
			continue
		}
		keyboard.AddRow()
		keyboard.AddTextButton(label, buttonPayload{Value: opt.Value}, vkobject.Primary)
	}

	return keyboard
}
