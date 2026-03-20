package model

// TextStyle controls how a text block is rendered by the platform adapter.
type TextStyle int

const (
	StylePlain TextStyle = iota
	StyleHeader
	StyleSubheader
	StyleCode
	StyleQuote
)

// ContentBlock is the sealed interface for message content elements.
// Only types in this package may implement it.
type ContentBlock interface {
	blockType() string
}

// TextBlock holds styled text.
type TextBlock struct {
	Text  string
	Style TextStyle
}

func (TextBlock) blockType() string { return "text" }

// OptionsBlock presents a prompt with selectable options.
type OptionsBlock struct {
	Prompt  string
	Options []Option
}

func (OptionsBlock) blockType() string { return "options" }

// Option is a single selectable choice within an OptionsBlock.
type Option struct {
	Label string
	Value string
}

// LinkBlock embeds a hyperlink.
type LinkBlock struct {
	URL   string
	Label string
}

func (LinkBlock) blockType() string { return "link" }

// ImageBlock embeds an image by URL.
type ImageBlock struct {
	URL string
}

func (ImageBlock) blockType() string { return "image" }

// MentionBlock references another user in a message.
type MentionBlock struct {
	UserID   string
	Username string
}

func (MentionBlock) blockType() string { return "mention" }

// Message is an ordered sequence of ContentBlock elements that platform
// adapters render into the native message format.
type Message struct {
	Blocks []ContentBlock
}

// NewTextMessage creates a Message containing a single plain-text block.
func NewTextMessage(text string) Message {
	return Message{
		Blocks: []ContentBlock{
			TextBlock{Text: text, Style: StylePlain},
		},
	}
}

// NewStyledTextMessage creates a Message containing a single styled text block.
func NewStyledTextMessage(text string, style TextStyle) Message {
	return Message{
		Blocks: []ContentBlock{
			TextBlock{Text: text, Style: style},
		},
	}
}

// NewOptionsMessage creates a Message containing a prompt with options.
func NewOptionsMessage(prompt string, options []Option) Message {
	return Message{
		Blocks: []ContentBlock{
			OptionsBlock{Prompt: prompt, Options: options},
		},
	}
}

// NewLinkMessage creates a Message containing a single link.
func NewLinkMessage(url, label string) Message {
	return Message{
		Blocks: []ContentBlock{
			LinkBlock{URL: url, Label: label},
		},
	}
}

// NewImageMessage creates a Message containing a single image.
func NewImageMessage(url string) Message {
	return Message{
		Blocks: []ContentBlock{
			ImageBlock{URL: url},
		},
	}
}
