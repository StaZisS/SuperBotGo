package discord

import (
	"fmt"
	"strings"

	"SuperBotGo/internal/model"
)

// Button represents a Discord message component button.
type Button struct {
	Label    string
	CustomID string
}

// RenderedMessage is the Discord-specific rendering of a model.Message.
type RenderedMessage struct {
	Text       string
	ImageURLs  []string
	HasOptions bool
	Buttons    [][]Button // rows of buttons, max 5 per row, max 5 rows
}

// Renderer converts model.Message to Discord RenderedMessage.
type Renderer struct{}

// NewRenderer creates a new Discord message renderer.
func NewRenderer() *Renderer {
	return &Renderer{}
}

// Render converts a model.Message into a Discord-specific RenderedMessage.
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

	return RenderedMessage{
		Text:       strings.Join(textParts, "\n"),
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

// buildOptionButtons groups options into rows of up to 5 buttons, max 5 rows.
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
