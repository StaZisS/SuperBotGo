package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync/atomic"
	"time"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/filestore"
	"SuperBotGo/internal/model"

	tele "gopkg.in/telebot.v3"
)

type BotConfig struct {
	Token         string
	Mode          string // "polling" (default) or "webhook"
	WebhookURL    string
	WebhookSecret string
	WebhookListen string
}

type Bot struct {
	bot          *tele.Bot
	handler      channel.UpdateHandlerFunc
	joinHandler  channel.ChatJoinHandler
	fileStore    filestore.FileStore
	maxFileSize  int64
	logger       *slog.Logger
	connected    atomic.Bool
	mode         string
	albums       *albumBuffer
	lifecycleCtx context.Context
}

func NewBot(cfg BotConfig, handler channel.UpdateHandlerFunc, joinHandler channel.ChatJoinHandler, fs filestore.FileStore, maxFileSize int64, logger *slog.Logger) (*Bot, error) {
	if logger == nil {
		logger = slog.Default()
	}

	mode := cfg.Mode
	if mode == "" {
		mode = "polling"
	}

	var poller tele.Poller
	switch mode {
	case "webhook":
		wh := &tele.Webhook{
			SecretToken: cfg.WebhookSecret,
			Endpoint:    &tele.WebhookEndpoint{PublicURL: cfg.WebhookURL},
		}
		if cfg.WebhookListen != "" {
			wh.Listen = cfg.WebhookListen
		}
		poller = wh
	default:
		poller = &tele.LongPoller{Timeout: 10 * time.Second}
	}

	pref := tele.Settings{
		Token:  cfg.Token,
		Poller: poller,
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		return nil, fmt.Errorf("telegram: create bot: %w", err)
	}

	tb := &Bot{
		bot:         b,
		handler:     handler,
		joinHandler: joinHandler,
		fileStore:   fs,
		maxFileSize: maxFileSize,
		logger:      logger,
		mode:        mode,
	}

	tb.albums = newAlbumBuffer(func(album *pendingAlbum) {
		tb.flushAlbum(album)
	})

	tb.registerHandlers()

	return tb, nil
}

func (b *Bot) Adapter() *Adapter {
	return NewAdapter(b.bot, &b.connected, b.fileStore)
}

func (b *Bot) Start(ctx context.Context) error {
	b.logger.Info("Telegram bot starting", slog.String("mode", b.mode))
	b.lifecycleCtx = ctx
	b.connected.Store(true)

	go func() {
		<-ctx.Done()
		b.logger.Info("Telegram bot stopping")
		b.connected.Store(false)
		b.bot.Stop()
	}()

	b.bot.Start()
	return nil
}

// deriveContext returns a context derived from the bot lifecycle context.
func (b *Bot) deriveContext() context.Context {
	if b.lifecycleCtx != nil {
		return b.lifecycleCtx
	}
	return context.Background()
}

func (b *Bot) Stop() {
	b.bot.Stop()
}

func (b *Bot) RegisterCommands(commands []string) {
	for _, cmd := range commands {
		name := cmd
		b.bot.Handle("/"+name, func(c tele.Context) error {
			return b.handleTextMessage(c)
		})
	}
}

func (b *Bot) handleTextMessage(c tele.Context) error {
	chatID := strconv.FormatInt(c.Chat().ID, 10)
	platformUserID := strconv.FormatInt(c.Sender().ID, 10)
	updateID := strconv.Itoa(c.Update().ID)
	text := c.Text()

	b.logger.Info("telegram: received message",
		slog.String("user", platformUserID),
		slog.String("chat", chatID),
		slog.String("text", text))

	ctx := b.deriveContext()
	if err := b.handler(ctx, channel.Update{
		ChannelType:      model.ChannelTelegram,
		PlatformUserID:   model.PlatformUserID(platformUserID),
		PlatformUpdateID: "tg:" + updateID,
		Input:            model.TextInput{Text: text},
		ChatID:           chatID,
		Username:         c.Sender().Username,
	}); err != nil {
		b.logger.Error("telegram: error handling message",
			slog.String("user", platformUserID),
			slog.Any("error", err))
	}
	return nil
}

func (b *Bot) handleFileMessage(c tele.Context, fileType model.FileType) error {
	// Skip if no fileStore configured
	if b.fileStore == nil {
		return nil
	}

	chatID := strconv.FormatInt(c.Chat().ID, 10)
	platformUserID := strconv.FormatInt(c.Sender().ID, 10)
	updateID := strconv.Itoa(c.Update().ID)

	// Extract the tele.File from the message based on type
	var teleFile *tele.File
	var filename string
	var mimeType string

	msg := c.Message()
	switch fileType {
	case model.FileTypePhoto:
		if msg.Photo != nil {
			teleFile = &msg.Photo.File
			filename = "photo.jpg"
			mimeType = "image/jpeg"
		}
	case model.FileTypeDocument:
		if msg.Document != nil {
			teleFile = &msg.Document.File
			filename = msg.Document.FileName
			mimeType = msg.Document.MIME
		}
	case model.FileTypeAudio:
		if msg.Audio != nil {
			teleFile = &msg.Audio.File
			filename = msg.Audio.FileName
			if filename == "" {
				filename = "audio"
			}
			mimeType = msg.Audio.MIME
		}
	case model.FileTypeVideo:
		if msg.Video != nil {
			teleFile = &msg.Video.File
			filename = msg.Video.FileName
			if filename == "" {
				filename = "video.mp4"
			}
			mimeType = msg.Video.MIME
		}
	case model.FileTypeVoice:
		if msg.Voice != nil {
			teleFile = &msg.Voice.File
			filename = "voice.ogg"
			mimeType = msg.Voice.MIME
		}
	}

	if teleFile == nil {
		return nil
	}

	// Check file size limit
	if b.maxFileSize > 0 && int64(teleFile.FileSize) > b.maxFileSize {
		b.logger.Warn("telegram: file too large, ignoring",
			slog.String("user", platformUserID),
			slog.Int64("file_size", teleFile.FileSize),
			slog.Int64("max_size", b.maxFileSize))
		return nil
	}

	// Download from Telegram
	reader, err := b.bot.File(teleFile)
	if err != nil {
		b.logger.Error("telegram: failed to download file",
			slog.String("user", platformUserID),
			slog.Any("error", err))
		return nil
	}
	defer reader.Close()

	// Store in FileStore
	ctx := b.deriveContext()
	ref, err := b.fileStore.Store(ctx, filestore.FileMeta{
		Name:     filename,
		MIMEType: mimeType,
		FileType: fileType,
	}, reader)
	if err != nil {
		b.logger.Error("telegram: failed to store file",
			slog.String("user", platformUserID),
			slog.Any("error", err))
		return nil
	}

	b.logger.Info("telegram: received file",
		slog.String("user", platformUserID),
		slog.String("chat", chatID),
		slog.String("file_id", ref.ID),
		slog.String("file_type", string(fileType)),
		slog.String("album_id", msg.AlbumID))

	caption := ""
	if msg.Caption != "" {
		caption = msg.Caption
	}

	entry := albumEntry{
		ref:      ref,
		caption:  caption,
		chatID:   chatID,
		userID:   platformUserID,
		updateID: updateID,
		username: c.Sender().Username,
	}

	// If part of an album (media group), buffer and wait for more files.
	if b.albums.add(msg.AlbumID, entry) {
		return nil
	}

	// Single file — dispatch immediately.
	if err := b.handler(ctx, channel.Update{
		ChannelType:      model.ChannelTelegram,
		PlatformUserID:   model.PlatformUserID(platformUserID),
		PlatformUpdateID: "tg:" + updateID,
		Input:            model.FileInput{Caption: caption, Files: []model.FileRef{ref}},
		ChatID:           chatID,
		Username:         c.Sender().Username,
	}); err != nil {
		b.logger.Error("telegram: error handling file message",
			slog.String("user", platformUserID),
			slog.Any("error", err))
	}
	return nil
}

// flushAlbum is called when an album's flush timer fires. It combines all
// buffered files into a single FileInput and dispatches it.
func (b *Bot) flushAlbum(album *pendingAlbum) {
	if len(album.entries) == 0 {
		return
	}

	first := album.entries[0]
	refs := make([]model.FileRef, len(album.entries))
	for i, e := range album.entries {
		refs[i] = e.ref
	}

	// Use caption from whichever entry has one (Telegram sets caption on first photo only).
	caption := ""
	for _, e := range album.entries {
		if e.caption != "" {
			caption = e.caption
			break
		}
	}

	b.logger.Info("telegram: flushing album",
		slog.String("user", first.userID),
		slog.String("chat", first.chatID),
		slog.Int("files", len(refs)))

	ctx := b.deriveContext()
	if err := b.handler(ctx, channel.Update{
		ChannelType:      model.ChannelTelegram,
		PlatformUserID:   model.PlatformUserID(first.userID),
		PlatformUpdateID: "tg:" + first.updateID,
		Input:            model.FileInput{Caption: caption, Files: refs},
		ChatID:           first.chatID,
		Username:         first.username,
	}); err != nil {
		b.logger.Error("telegram: error handling album",
			slog.String("user", first.userID),
			slog.Any("error", err))
	}
}

func (b *Bot) handleMyChatMember(c tele.Context) error {
	if b.joinHandler == nil {
		return nil
	}

	update := c.ChatMember()
	if update == nil {
		return nil
	}

	chat := update.Chat
	if chat == nil {
		return nil
	}

	chatID := strconv.FormatInt(chat.ID, 10)
	newStatus := update.NewChatMember.Role

	if newStatus == tele.Left || newStatus == tele.Kicked {
		b.logger.Info("telegram: bot removed from chat",
			slog.String("chat_id", chatID),
			slog.String("chat_type", string(chat.Type)),
			slog.String("new_status", string(newStatus)))

		ctx := b.deriveContext()
		if err := b.joinHandler.OnChatLeave(ctx, model.ChannelTelegram, chatID); err != nil {
			b.logger.Error("telegram: failed to unregister chat on leave",
				slog.String("chat_id", chatID),
				slog.Any("error", err))
		}
		return nil
	}

	var chatKind model.ChatKind
	switch chat.Type {
	case tele.ChatGroup, tele.ChatSuperGroup:
		chatKind = model.ChatKindGroup
	case tele.ChatChannel:
		chatKind = model.ChatKindChannel
	case tele.ChatPrivate:
		chatKind = model.ChatKindPrivate
	default:
		chatKind = model.ChatKindGroup
	}

	title := chat.Title
	if title == "" && chat.Type == tele.ChatPrivate {
		title = chat.FirstName
		if chat.LastName != "" {
			title += " " + chat.LastName
		}
	}

	b.logger.Info("telegram: bot added to chat",
		slog.String("chat_id", chatID),
		slog.String("chat_type", string(chat.Type)),
		slog.String("title", title),
		slog.String("new_status", string(newStatus)))

	ctx := b.deriveContext()
	if err := b.joinHandler.OnChatJoin(ctx, model.ChannelTelegram, chatID, chatKind, title); err != nil {
		b.logger.Error("telegram: failed to register chat on join",
			slog.String("chat_id", chatID),
			slog.Any("error", err))
	}
	return nil
}

func (b *Bot) registerHandlers() {

	b.bot.Handle(tele.OnText, func(c tele.Context) error {
		return b.handleTextMessage(c)
	})

	b.bot.Handle(tele.OnPhoto, func(c tele.Context) error {
		return b.handleFileMessage(c, model.FileTypePhoto)
	})
	b.bot.Handle(tele.OnDocument, func(c tele.Context) error {
		return b.handleFileMessage(c, model.FileTypeDocument)
	})
	b.bot.Handle(tele.OnAudio, func(c tele.Context) error {
		return b.handleFileMessage(c, model.FileTypeAudio)
	})
	b.bot.Handle(tele.OnVideo, func(c tele.Context) error {
		return b.handleFileMessage(c, model.FileTypeVideo)
	})
	b.bot.Handle(tele.OnVoice, func(c tele.Context) error {
		return b.handleFileMessage(c, model.FileTypeVoice)
	})

	b.bot.Handle(tele.OnMyChatMember, func(c tele.Context) error {
		return b.handleMyChatMember(c)
	})

	b.bot.Handle(tele.OnCallback, func(c tele.Context) error {
		_ = c.Respond()

		chatID := strconv.FormatInt(c.Chat().ID, 10)
		platformUserID := strconv.FormatInt(c.Sender().ID, 10)
		updateID := strconv.Itoa(c.Update().ID)
		data := c.Callback().Data

		b.logger.Info("telegram: received callback",
			slog.String("user", platformUserID),
			slog.String("chat", chatID),
			slog.String("data", data))

		ctx := b.deriveContext()
		if err := b.handler(ctx, channel.Update{
			ChannelType:      model.ChannelTelegram,
			PlatformUserID:   model.PlatformUserID(platformUserID),
			PlatformUpdateID: "tg:" + updateID,
			Input:            model.CallbackInput{Data: data},
			ChatID:           chatID,
			Username:         c.Sender().Username,
		}); err != nil {
			b.logger.Error("telegram: error handling callback",
				slog.String("user", platformUserID),
				slog.Any("error", err))
		}
		return nil
	})
}
