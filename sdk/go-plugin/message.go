package wasmplugin

// Message represents a rich notification/reply message composed of content blocks.
// Each block can optionally carry localized text variants.
//
// Create with [NewMessage] or [NewLocalizedMessage] and extend with builder methods.
//
// Usage:
//
//	// Simple text
//	wasmplugin.NewMessage("Лекция отменена")
//
//	// Localized text (via Catalog)
//	wasmplugin.NewLocalizedMessage(catalog.L("lecture_cancelled"))
//
//	// Rich content
//	wasmplugin.NewMessage("Замена преподавателя").
//	    Mention(newTeacherID).
//	    Link("https://schedule.university.ru/changes", "Подробности")
type Message struct {
	blocks []msgBlock
}

type msgBlock struct {
	Type    string            `json:"type" msgpack:"type"`
	Text    string            `json:"text,omitempty" msgpack:"text,omitempty"`
	Texts   map[string]string `json:"texts,omitempty" msgpack:"texts,omitempty"`
	Style   string            `json:"style,omitempty" msgpack:"style,omitempty"`
	UserID  string            `json:"user_id,omitempty" msgpack:"user_id,omitempty"`
	FileID  string            `json:"file_id,omitempty" msgpack:"file_id,omitempty"`
	Caption string            `json:"caption,omitempty" msgpack:"caption,omitempty"`
	URL     string            `json:"url,omitempty" msgpack:"url,omitempty"`
	Label   string            `json:"label,omitempty" msgpack:"label,omitempty"`
}

// NewMessage creates a Message with a single plain text block.
func NewMessage(text string) Message {
	return Message{blocks: []msgBlock{{Type: "text", Text: text, Style: string(StylePlain)}}}
}

// NewLocalizedMessage creates a Message with a single localized text block.
// The host resolves the locale per-recipient. Use with [Catalog.L]:
//
//	wasmplugin.NewLocalizedMessage(catalog.L("greeting"))
func NewLocalizedMessage(texts map[string]string) Message {
	return Message{blocks: []msgBlock{{Type: "text", Texts: texts, Style: string(StylePlain)}}}
}

// Text appends a plain text block.
func (m Message) Text(text string) Message {
	m.blocks = append(m.blocks, msgBlock{Type: "text", Text: text, Style: string(StylePlain)})
	return m
}

// LocalizedText appends a localized text block.
func (m Message) LocalizedText(texts map[string]string) Message {
	m.blocks = append(m.blocks, msgBlock{Type: "text", Texts: texts, Style: string(StylePlain)})
	return m
}

// StyledText appends a text block with the given style.
func (m Message) StyledText(text string, style TextStyle) Message {
	m.blocks = append(m.blocks, msgBlock{Type: "text", Text: text, Style: string(style)})
	return m
}

// LocalizedStyledText appends a localized text block with the given style.
func (m Message) LocalizedStyledText(texts map[string]string, style TextStyle) Message {
	m.blocks = append(m.blocks, msgBlock{Type: "text", Texts: texts, Style: string(style)})
	return m
}

// Mention appends a user mention block.
func (m Message) Mention(userID string) Message {
	m.blocks = append(m.blocks, msgBlock{Type: "mention", UserID: userID})
	return m
}

// File appends a file attachment block.
func (m Message) File(ref FileRef, caption string) Message {
	m.blocks = append(m.blocks, msgBlock{Type: "file", FileID: ref.ID, Caption: caption})
	return m
}

// Link appends a link block.
func (m Message) Link(url, label string) Message {
	m.blocks = append(m.blocks, msgBlock{Type: "link", URL: url, Label: label})
	return m
}

// Image appends an image block.
func (m Message) Image(url string) Message {
	m.blocks = append(m.blocks, msgBlock{Type: "image", URL: url})
	return m
}

// IsEmpty returns true if the message contains no blocks.
func (m Message) IsEmpty() bool {
	return len(m.blocks) == 0
}
