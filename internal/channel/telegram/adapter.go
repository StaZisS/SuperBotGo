package telegram

import (
	"context"
	"fmt"
	"io"
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

const telegramCaptionMaxLength = 1024

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

	// Collect photos into an album and non-photo files separately.
	var album tele.Album
	var closers []io.Closer
	defer func() {
		for _, c := range closers {
			_ = c.Close()
		}
	}()

	type docEntry struct {
		name     string
		mimeType string
		reader   io.ReadCloser
	}
	var docs []docEntry

	for _, photoURL := range rendered.PhotoURLs {
		album = append(album, &tele.Photo{File: tele.FromURL(photoURL)})
	}

	if a.fileStore != nil {
		for _, ref := range rendered.FileRefs {
			reader, meta, fErr := a.fileStore.Get(context.Background(), ref.ID)
			if fErr != nil {
				return fmt.Errorf("telegram: get file %q: %w", ref.ID, fErr)
			}
			closers = append(closers, reader)

			name := ref.Name
			mimeType := ref.MIMEType
			fileType := ref.FileType
			if meta != nil {
				if name == "" {
					name = meta.Name
				}
				if mimeType == "" {
					mimeType = meta.MIMEType
				}
				if fileType == "" {
					fileType = meta.FileType
				}
			}

			if fileType == model.FileTypePhoto {
				album = append(album, &tele.Photo{File: tele.FromReader(reader)})
			} else {
				docs = append(docs, docEntry{name: name, mimeType: mimeType, reader: reader})
			}
		}
	}

	// If there are photos, no keyboard, and text fits in a caption —
	// embed text as the album caption instead of sending separately.
	hasKeyboard := len(rendered.Keyboard) > 0
	textAsCaption := len(album) > 0 && !hasKeyboard &&
		rendered.Text != "" && len([]rune(rendered.Text)) <= telegramCaptionMaxLength
	if textAsCaption {
		if p, ok := album[0].(*tele.Photo); ok {
			p.Caption = rendered.Text
		}
	}

	// Send text as a separate message when it wasn't used as caption.
	if rendered.Text != "" && !textAsCaption {
		opts := &tele.SendOptions{
			ParseMode:           tele.ModeHTML,
			DisableNotification: silent,
		}

		if hasKeyboard {
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

	// Send photos: single photo via Send, multiple via SendAlbum.
	if len(album) == 1 {
		opts := &tele.SendOptions{
			ParseMode:           tele.ModeHTML,
			DisableNotification: silent,
		}
		if _, err := a.bot.Send(recipient, album[0], opts); err != nil {
			return fmt.Errorf("telegram: send photo: %w", err)
		}
	} else if len(album) > 1 {
		opts := &tele.SendOptions{
			ParseMode:           tele.ModeHTML,
			DisableNotification: silent,
		}
		if _, err := a.bot.SendAlbum(recipient, album, opts); err != nil {
			return fmt.Errorf("telegram: send album: %w", err)
		}
	}

	// Send non-photo files individually.
	for _, d := range docs {
		doc := &tele.Document{
			File:     tele.FromReader(d.reader),
			FileName: d.name,
			MIME:     d.mimeType,
		}
		if _, err := a.bot.Send(recipient, doc); err != nil {
			return fmt.Errorf("telegram: send file: %w", err)
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
