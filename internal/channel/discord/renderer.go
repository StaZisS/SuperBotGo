package discord

import (
	"fmt"
	"strings"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/model"
)

type Button struct {
	Label    string
	CustomID string
}

type RenderedMessage struct {
	Text       string
	ImageURLs  []string
	HasOptions bool
	Buttons    [][]Button
	FileRefs   []model.FileRef
}

type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

const discordMaxMessageLength = 2000

type formatter struct{}

func (formatter) FormatText(b model.TextBlock) string {
	switch b.Style {
	case model.StyleHeader:
		return fmt.Sprintf("# %s", b.Text)
	case model.StyleSubheader:
		return fmt.Sprintf("## %s", b.Text)
	case model.StyleCode:
		return fmt.Sprintf("```\n%s\n```", b.Text)
	case model.StyleQuote:
		return fmt.Sprintf("> %s", b.Text)
	default:
		return b.Text
	}
}

func (formatter) FormatMention(b model.MentionBlock) string {
	return fmt.Sprintf("<@%s>", b.UserID)
}

func (formatter) FormatLink(b model.LinkBlock) string {
	label := b.Label
	if label == "" {
		label = b.URL
	}
	return fmt.Sprintf("[%s](%s)", label, b.URL)
}

func (formatter) FormatOptionsPrompt(prompt string) string {
	return prompt
}

func (r *Renderer) Render(msg model.Message) RenderedMessage {
	parts := channel.RenderBlocks(msg, formatter{})

	var buttons [][]Button
	if parts.Options != nil {
		buttons = buildOptionButtons(parts.Options.Options)
	}

	text := strings.Join(parts.TextParts, "\n")
	if len([]rune(text)) > discordMaxMessageLength {
		runes := []rune(text)
		text = string(runes[:discordMaxMessageLength-3]) + "..."
	}

	return RenderedMessage{
		Text:       text,
		ImageURLs:  parts.ImageURLs,
		HasOptions: len(buttons) > 0,
		Buttons:    buttons,
		FileRefs:   parts.FileRefs,
	}
}

func buildOptionButtons(options []model.Option) [][]Button {
	const maxPerRow = 5
	const maxRows = 5

	var rows [][]Button
	for i := 0; i < len(options); i += maxPerRow {
		if len(rows) >= maxRows {
			break
		}
		end := i + maxPerRow
		if end > len(options) {
			end = len(options)
		}
		row := make([]Button, 0, end-i)
		for _, opt := range options[i:end] {
			label := channel.OptionLabel(opt)
			if label == "" {
				continue
			}
			row = append(row, Button{
				Label:    label,
				CustomID: opt.Value,
			})
		}
		if len(row) > 0 {
			rows = append(rows, row)
		}
	}
	return rows
}
