package telegram

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/filestore"
	"SuperBotGo/internal/model"

	tele "gopkg.in/telebot.v3"
)

var (
	_ channel.SilentSender  = (*Adapter)(nil)
	_ channel.StatusChecker = (*Adapter)(nil)
)

type Adapter struct {
	bot       *tele.Bot
	renderer  *Renderer
	connected *atomic.Bool
	fileStore filestore.FileStore
}

func NewAdapter(bot *tele.Bot, connected *atomic.Bool, fs filestore.FileStore) *Adapter {
	return &Adapter{
		bot:       bot,
		renderer:  NewRenderer(),
		connected: connected,
		fileStore: fs,
	}
}

func (a *Adapter) Connected() bool {
	return a.connected.Load()
}

func (a *Adapter) Type() model.ChannelType {
	return model.ChannelTelegram
}

func (a *Adapter) SendToUser(ctx context.Context, platformUserID model.PlatformUserID, msg model.Message) error {
	return a.sendMessage(ctx, string(platformUserID), msg, false)
}

func (a *Adapter) SendToChat(ctx context.Context, chatID string, msg model.Message) error {
	return a.sendMessage(ctx, chatID, msg, false)
}

func (a *Adapter) SendToUserSilent(ctx context.Context, platformUserID model.PlatformUserID, msg model.Message, silent bool) error {
	return a.sendMessage(ctx, string(platformUserID), msg, silent)
}

func (a *Adapter) SendToChatSilent(ctx context.Context, chatID string, msg model.Message, silent bool) error {
	return a.sendMessage(ctx, chatID, msg, silent)
}

func (a *Adapter) sendMessage(_ context.Context, chatID string, msg model.Message, silent bool) error {
	if msg.IsEmpty() {
		return fmt.Errorf("telegram: refusing to send empty message to chat %s", chatID)
	}

	rendered := a.renderer.Render(msg)

	if rendered.Text == "" && len(rendered.PhotoURLs) == 0 && len(rendered.FileRefs) == 0 {
		return nil // nothing to send after rendering
	}

	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram: invalid chat ID %q: %w", chatID, err)
	}

	recipient := &telegramChat{id: id}

	if rendered.Text != "" {
		opts := &tele.SendOptions{
			ParseMode:           tele.ModeHTML,
			DisableNotification: silent,
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

	if a.fileStore != nil {
		for _, ref := range rendered.FileRefs {
			reader, _, fErr := a.fileStore.Get(context.Background(), ref.ID)
			if fErr != nil {
				return fmt.Errorf("telegram: get file %q: %w", ref.ID, fErr)
			}
			doc := &tele.Document{
				File:     tele.FromReader(reader),
				FileName: ref.Name,
				MIME:     ref.MIMEType,
			}
			if _, fErr = a.bot.Send(recipient, doc); fErr != nil {
				_ = reader.Close()
				return fmt.Errorf("telegram: send file: %w", fErr)
			}
			_ = reader.Close()
		}
	}

	return nil
}

type telegramChat struct {
	id int64
}

func (c *telegramChat) Recipient() string {
	return strconv.FormatInt(c.id, 10)
}
