package telegram

import (
	"context"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/model"
)

func CallbackNormalizer() channel.UpdateMiddleware {
	return func(next channel.UpdateHandlerFunc) channel.UpdateHandlerFunc {
		return func(ctx context.Context, u channel.Update) error {
			if cb, ok := u.Input.(model.CallbackInput); ok {
				u.Input = model.CallbackInput{
					Data:  stripTelebotPrefix(cb.Data),
					Label: cb.Label,
				}
			}
			return next(ctx, u)
		}
	}
}

func stripTelebotPrefix(data string) string {
	if len(data) == 0 || data[0] != '\f' {
		return data
	}
	rest := data[1:]
	if idx := indexOf(rest, '|'); idx >= 0 {
		return rest[idx+1:]
	}
	if idx := indexOf(rest, '\f'); idx >= 0 {
		return rest[idx+1:]
	}
	return rest
}

func indexOf(s string, ch byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ch {
			return i
		}
	}
	return -1
}
