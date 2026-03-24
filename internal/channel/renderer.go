package channel

import "SuperBotGo/internal/model"

// BlockFormatter defines platform-specific formatting for message content blocks.
type BlockFormatter interface {
	FormatText(model.TextBlock) string
	FormatMention(model.MentionBlock) string
	FormatLink(model.LinkBlock) string
	FormatOptionsPrompt(prompt string) string
}

// RenderParts holds the intermediate result of rendering message blocks,
// before platform-specific assembly into the final message struct.
type RenderParts struct {
	TextParts []string
	ImageURLs []string
	Options   *model.OptionsBlock
}

// RenderBlocks iterates over message blocks and formats them using the
// provided BlockFormatter, returning intermediate parts that each platform
// renderer can assemble into its own output type.
func RenderBlocks(msg model.Message, f BlockFormatter) RenderParts {
	var parts RenderParts
	for _, block := range msg.Blocks {
		switch b := block.(type) {
		case model.TextBlock:
			parts.TextParts = append(parts.TextParts, f.FormatText(b))
		case model.MentionBlock:
			parts.TextParts = append(parts.TextParts, f.FormatMention(b))
		case model.LinkBlock:
			parts.TextParts = append(parts.TextParts, f.FormatLink(b))
		case model.ImageBlock:
			parts.ImageURLs = append(parts.ImageURLs, b.URL)
		case model.OptionsBlock:
			if b.Prompt != "" {
				parts.TextParts = append(parts.TextParts, f.FormatOptionsPrompt(b.Prompt))
			}
			opts := b
			parts.Options = &opts
		}
	}
	return parts
}
