package channel

import "SuperBotGo/internal/model"

// RenderedMessage is a platform-agnostic rendered message returned by renderers.
// Platform-specific renderers embed or extend this with additional fields.
type RenderedMessage struct {
	Text string
}

// MessageRenderer converts a model.Message into a platform-specific rendered form.
type MessageRenderer interface {
	Render(msg model.Message) (RenderedMessage, error)
}
