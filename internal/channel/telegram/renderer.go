package telegram

import (
	"fmt"
	"strings"

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
}

type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

const telegramMaxMessageLength = 4096

func (r *Renderer) Render(msg model.Message) RenderedMessage {
	var textParts []string
	var photoURLs []string
	var keyboard [][]InlineButton

	for _, block := range msg.Blocks {
		switch b := block.(type) {
		case model.TextBlock:
			textParts = append(textParts, renderTextBlock(b))
		case model.MentionBlock:
			textParts = append(textParts, renderMentionBlock(b))
		case model.LinkBlock:
			textParts = append(textParts, renderLinkBlock(b))
		case model.ImageBlock:
			photoURLs = append(photoURLs, b.URL)
		case model.OptionsBlock:
			if b.Prompt != "" {
				textParts = append(textParts, escapeHTML(b.Prompt))
			}
			keyboard = buildOptionsKeyboard(b)
		}
	}

	text := strings.Join(textParts, "\n")
	if len([]rune(text)) > telegramMaxMessageLength {
		runes := []rune(text)
		text = string(runes[:telegramMaxMessageLength-3]) + "..."
	}

	return RenderedMessage{
		Text:      text,
		ParseMode: "HTML",
		Keyboard:  keyboard,
		PhotoURLs: photoURLs,
	}
}

func renderTextBlock(b model.TextBlock) string {
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

func renderMentionBlock(b model.MentionBlock) string {
	name := escapeHTML(b.Username)
	return fmt.Sprintf(`<a href="tg://user?id=%s">%s</a>`, b.UserID, name)
}

func renderLinkBlock(b model.LinkBlock) string {
	label := escapeHTML(b.Label)
	return fmt.Sprintf(`<a href="%s">%s</a>`, b.URL, label)
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
