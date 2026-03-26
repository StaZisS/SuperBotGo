package channel

import (
	"context"

	"SuperBotGo/internal/model"
)

type Update struct {
	ChannelType      model.ChannelType
	PlatformUserID   model.PlatformUserID
	PlatformUpdateID string
	Input            model.UserInput
	ChatID           string
}

type UpdateHandlerFunc func(ctx context.Context, u Update) error

type UpdateMiddleware func(next UpdateHandlerFunc) UpdateHandlerFunc

func Chain(handler UpdateHandlerFunc, mw ...UpdateMiddleware) UpdateHandlerFunc {
	for i := len(mw) - 1; i >= 0; i-- {
		handler = mw[i](handler)
	}
	return handler
}
