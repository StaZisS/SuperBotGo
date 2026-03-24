package discord

import (
	"fmt"
	"strings"

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
}

type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

const discordMaxMessageLength = 2000

func (r *Renderer) Render(msg model.Message) RenderedMessage {
	var textParts []string
	var imageURLs []string
	var buttons [][]Button

	for _, block := range msg.Blocks {
		switch b := block.(type) {
		case model.TextBlock:
			textParts = append(textParts, renderTextBlock(b))
		case model.MentionBlock:
			textParts = append(textParts, renderMentionBlock(b))
		case model.LinkBlock:
			textParts = append(textParts, renderLinkBlock(b))
		case model.ImageBlock:
			imageURLs = append(imageURLs, b.URL)
		case model.OptionsBlock:
			if b.Prompt != "" {
				textParts = append(textParts, b.Prompt)
			}
			buttons = buildOptionButtons(b.Options)
		}
	}

	text := strings.Join(textParts, "\n")
	if len(text) > discordMaxMessageLength {
		text = text[:discordMaxMessageLength-3] + "..."
	}

	return RenderedMessage{
		Text:       text,
		ImageURLs:  imageURLs,
		HasOptions: len(buttons) > 0,
		Buttons:    buttons,
	}
}

func renderTextBlock(b model.TextBlock) string {
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

func renderMentionBlock(b model.MentionBlock) string {
	return fmt.Sprintf("<@%s>", b.UserID)
}

func renderLinkBlock(b model.LinkBlock) string {
	return fmt.Sprintf("[%s](%s)", b.Label, b.URL)
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
			row = append(row, Button{
				Label:    opt.Label,
				CustomID: opt.Value,
			})
		}
		rows = append(rows, row)
	}
	return rows
}
