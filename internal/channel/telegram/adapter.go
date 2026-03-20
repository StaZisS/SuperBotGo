package telegram

import (
	"context"
	"fmt"
	"strconv"

	"SuperBotGo/internal/model"

	tele "gopkg.in/telebot.v3"
)

// Adapter implements channel.ChannelAdapter for Telegram.
type Adapter struct {
	bot      *tele.Bot
	renderer *Renderer
}

// NewAdapter creates a Telegram adapter wrapping the given telebot instance.
func NewAdapter(bot *tele.Bot) *Adapter {
	return &Adapter{
		bot:      bot,
		renderer: NewRenderer(),
	}
}

// Type returns the Telegram channel type.
func (a *Adapter) Type() model.ChannelType {
	return model.ChannelTelegram
}

// SendToUser sends a message to a user by their Telegram user ID.
func (a *Adapter) SendToUser(ctx context.Context, platformUserID model.PlatformUserID, msg model.Message) error {
	return a.sendMessage(ctx, string(platformUserID), msg)
}

// SendToChat sends a message to a Telegram chat by its chat ID.
func (a *Adapter) SendToChat(ctx context.Context, chatID string, msg model.Message) error {
	return a.sendMessage(ctx, chatID, msg)
}

func (a *Adapter) sendMessage(_ context.Context, chatID string, msg model.Message) error {
	rendered := a.renderer.Render(msg)

	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram: invalid chat ID %q: %w", chatID, err)
	}

	recipient := &telegramChat{id: id}

	if rendered.Text != "" {
		opts := &tele.SendOptions{
			ParseMode: tele.ModeHTML,
		}

		if len(rendered.Keyboard) > 0 {
			rows := make([]tele.Row, 0, len(rendered.Keyboard))
			markup := &tele.ReplyMarkup{}
			for _, kbRow := range rendered.Keyboard {
				btns := make([]tele.Btn, 0, len(kbRow))
				for _, btn := range kbRow {
					btns = append(btns, markup.Data(btn.Text, btn.CallbackData, btn.CallbackData))
				}
				rows = append(rows, markup.Row(btns...))
			}
			markup.Inline(rows...)
			opts.ReplyMarkup = markup
		}

		if _, err := a.bot.Send(recipient, rendered.Text, opts); err != nil {
			return fmt.Errorf("telegram: send text: %w", err)
		}
	}

	for _, photoURL := range rendered.PhotoURLs {
		photo := &tele.Photo{File: tele.FromURL(photoURL)}
		if _, err := a.bot.Send(recipient, photo); err != nil {
			return fmt.Errorf("telegram: send photo: %w", err)
		}
	}

	return nil
}

// telegramChat implements telebot's Recipient interface.
type telegramChat struct {
	id int64
}

func (c *telegramChat) Recipient() string {
	return strconv.FormatInt(c.id, 10)
}
