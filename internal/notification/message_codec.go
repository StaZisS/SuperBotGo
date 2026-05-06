package notification

import (
	"encoding/json"
	"fmt"

	"SuperBotGo/internal/model"
)

type persistedMessage struct {
	Blocks []persistedBlock `json:"blocks"`
}

type persistedBlock struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	Style    model.TextStyle `json:"style,omitempty"`
	UserID   string          `json:"user_id,omitempty"`
	Username string          `json:"username,omitempty"`
	FileRef  model.FileRef   `json:"file_ref,omitempty"`
	Caption  string          `json:"caption,omitempty"`
	URL      string          `json:"url,omitempty"`
	Label    string          `json:"label,omitempty"`
	Prompt   string          `json:"prompt,omitempty"`
	Options  []model.Option  `json:"options,omitempty"`
}

func marshalMessage(msg model.Message) ([]byte, error) {
	out := persistedMessage{
		Blocks: make([]persistedBlock, 0, len(msg.Blocks)),
	}
	for _, block := range msg.Blocks {
		switch b := block.(type) {
		case model.TextBlock:
			out.Blocks = append(out.Blocks, persistedBlock{Type: "text", Text: b.Text, Style: b.Style})
		case model.OptionsBlock:
			out.Blocks = append(out.Blocks, persistedBlock{Type: "options", Prompt: b.Prompt, Options: b.Options})
		case model.LinkBlock:
			out.Blocks = append(out.Blocks, persistedBlock{Type: "link", URL: b.URL, Label: b.Label})
		case model.ImageBlock:
			out.Blocks = append(out.Blocks, persistedBlock{Type: "image", URL: b.URL})
		case model.MentionBlock:
			out.Blocks = append(out.Blocks, persistedBlock{Type: "mention", UserID: b.UserID, Username: b.Username})
		case model.FileBlock:
			out.Blocks = append(out.Blocks, persistedBlock{Type: "file", FileRef: b.FileRef, Caption: b.Caption})
		default:
			return nil, fmt.Errorf("notification: unsupported message block %T", block)
		}
	}
	return json.Marshal(out)
}

func unmarshalMessage(data []byte) (model.Message, error) {
	var in persistedMessage
	if err := json.Unmarshal(data, &in); err != nil {
		return model.Message{}, err
	}

	blocks := make([]model.ContentBlock, 0, len(in.Blocks))
	for _, block := range in.Blocks {
		switch block.Type {
		case "text":
			blocks = append(blocks, model.TextBlock{Text: block.Text, Style: block.Style})
		case "options":
			blocks = append(blocks, model.OptionsBlock{Prompt: block.Prompt, Options: block.Options})
		case "link":
			blocks = append(blocks, model.LinkBlock{URL: block.URL, Label: block.Label})
		case "image":
			blocks = append(blocks, model.ImageBlock{URL: block.URL})
		case "mention":
			blocks = append(blocks, model.MentionBlock{UserID: block.UserID, Username: block.Username})
		case "file":
			blocks = append(blocks, model.FileBlock{FileRef: block.FileRef, Caption: block.Caption})
		default:
			return model.Message{}, fmt.Errorf("notification: unknown persisted message block type %q", block.Type)
		}
	}
	return model.Message{Blocks: blocks}, nil
}
