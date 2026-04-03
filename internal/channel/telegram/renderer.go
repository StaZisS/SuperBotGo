package telegram

import (
	"fmt"
	"strings"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/model"
)

type InlineButton struct {
	Text         string
	CallbackData string
}

type RenderedMessage struct {
	Text      string
	ParseMode string
	Keyboard  [][]InlineButton
	PhotoURLs []string
	FileRefs  []model.FileRef
}

type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

const telegramMaxMessageLength = 4096

type formatter struct{}

func (formatter) FormatText(b model.TextBlock) string {
	escaped := escapeHTML(b.Text)
	switch b.Style {
	case model.StyleHeader:
		return fmt.Sprintf("<b>%s</b>", escaped)
	case model.StyleSubheader:
		return fmt.Sprintf("<b><i>%s</i></b>", escaped)
	case model.StyleCode:
		return fmt.Sprintf("<code>%s</code>", escaped)
	case model.StyleQuote:
		return fmt.Sprintf("<blockquote>%s</blockquote>", escaped)
	default:
		return escaped
	}
}

func (formatter) FormatMention(b model.MentionBlock) string {
	name := escapeHTML(b.Username)
	return fmt.Sprintf(`<a href="tg://user?id=%s">%s</a>`, b.UserID, name)
}

func (formatter) FormatLink(b model.LinkBlock) string {
	label := escapeHTML(b.Label)
	return fmt.Sprintf(`<a href="%s">%s</a>`, b.URL, label)
}

func (formatter) FormatOptionsPrompt(prompt string) string {
	return escapeHTML(prompt)
}

func (r *Renderer) Render(msg model.Message) RenderedMessage {
	parts := channel.RenderBlocks(msg, formatter{})

	var keyboard [][]InlineButton
	if parts.Options != nil {
		keyboard = buildOptionsKeyboard(*parts.Options)
	}

	text := strings.Join(parts.TextParts, "\n")
	if len([]rune(text)) > telegramMaxMessageLength {
		runes := []rune(text)
		text = string(runes[:telegramMaxMessageLength-3]) + "..."
	}

	return RenderedMessage{
		Text:      text,
		ParseMode: "HTML",
		Keyboard:  keyboard,
		PhotoURLs: parts.ImageURLs,
		FileRefs:  parts.FileRefs,
	}
}

func buildOptionsKeyboard(b model.OptionsBlock) [][]InlineButton {
	keyboard := make([][]InlineButton, 0, len(b.Options))
	for _, opt := range b.Options {
		row := []InlineButton{
			{Text: opt.Label, CallbackData: opt.Value},
		}
		keyboard = append(keyboard, row)
	}
	return keyboard
}

func escapeHTML(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	return text
}
