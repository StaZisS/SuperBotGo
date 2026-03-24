package model

type TextStyle int

const (
	StylePlain TextStyle = iota
	StyleHeader
	StyleSubheader
	StyleCode
	StyleQuote
)

type ContentBlock interface {
	blockType() string
}

type TextBlock struct {
	Text  string
	Style TextStyle
}

func (TextBlock) blockType() string { return "text" }

type OptionsBlock struct {
	Prompt  string
	Options []Option
}

func (OptionsBlock) blockType() string { return "options" }

type Option struct {
	Label string
	Value string
}

type LinkBlock struct {
	URL   string
	Label string
}

func (LinkBlock) blockType() string { return "link" }

type ImageBlock struct {
	URL string
}

func (ImageBlock) blockType() string { return "image" }

type MentionBlock struct {
	UserID   string
	Username string
}

func (MentionBlock) blockType() string { return "mention" }

type Message struct {
	Blocks []ContentBlock
}

// IsEmpty returns true if the message contains no content blocks.
func (m Message) IsEmpty() bool {
	return len(m.Blocks) == 0
}

func NewTextMessage(text string) Message {
	return Message{
		Blocks: []ContentBlock{
			TextBlock{Text: text, Style: StylePlain},
		},
	}
}

func NewStyledTextMessage(text string, style TextStyle) Message {
	return Message{
		Blocks: []ContentBlock{
			TextBlock{Text: text, Style: style},
		},
	}
}

func NewOptionsMessage(prompt string, options []Option) Message {
	return Message{
		Blocks: []ContentBlock{
			OptionsBlock{Prompt: prompt, Options: options},
		},
	}
}

func NewLinkMessage(url, label string) Message {
	return Message{
		Blocks: []ContentBlock{
			LinkBlock{URL: url, Label: label},
		},
	}
}

func NewImageMessage(url string) Message {
	return Message{
		Blocks: []ContentBlock{
			ImageBlock{URL: url},
		},
	}
}
