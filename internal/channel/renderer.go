package channel

import "SuperBotGo/internal/model"

type RenderedMessage struct {
	Text string
}

type MessageRenderer interface {
	Render(msg model.Message) (RenderedMessage, error)
}
