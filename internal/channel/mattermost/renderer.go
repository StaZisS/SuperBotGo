package mattermost

import (
	"fmt"
	"strings"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/model"
)

type RenderedMessage struct {
	Text      string
	ImageURLs []string
	FileRefs  []model.FileRef
	Options   *model.OptionsBlock
}

type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

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
	if b.Username != "" {
		return "@" + b.Username
	}
	return b.UserID
}

func (formatter) FormatLink(b model.LinkBlock) string {
	if b.Label == "" || b.Label == b.URL {
		return b.URL
	}
	return fmt.Sprintf("[%s](%s)", b.Label, b.URL)
}

func (formatter) FormatOptionsPrompt(prompt string) string {
	return prompt
}

func (r *Renderer) Render(msg model.Message) RenderedMessage {
	parts := channel.RenderBlocks(msg, formatter{})

	return RenderedMessage{
		Text:      strings.Join(parts.TextParts, "\n"),
		ImageURLs: parts.ImageURLs,
		FileRefs:  parts.FileRefs,
		Options:   parts.Options,
	}
}

func renderOptions(options []model.Option) []string {
	lines := make([]string, 0, len(options))
	for _, opt := range options {
		label := channel.OptionLabel(opt)
		switch {
		case label == "" || label == opt.Value:
			lines = append(lines, fmt.Sprintf("- `%s`", opt.Value))
		default:
			lines = append(lines, fmt.Sprintf("- %s: `%s`", label, opt.Value))
		}
	}
	return lines
}
