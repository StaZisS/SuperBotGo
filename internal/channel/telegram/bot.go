package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync/atomic"
	"time"

	"SuperBotGo/internal/channel"
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
	bot         *tele.Bot
	handler     channel.UpdateHandlerFunc
	joinHandler channel.ChatJoinHandler
	logger      *slog.Logger
	connected   atomic.Bool
	mode        string
}

func NewBot(cfg BotConfig, handler channel.UpdateHandlerFunc, joinHandler channel.ChatJoinHandler, logger *slog.Logger) (*Bot, error) {
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
		logger:      logger,
		mode:        mode,
	}

	tb.registerHandlers()

	return tb, nil
}

func (b *Bot) Adapter() *Adapter {
	return NewAdapter(b.bot, &b.connected)
}

func (b *Bot) Start(ctx context.Context) error {
	b.logger.Info("Telegram bot starting", slog.String("mode", b.mode))
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

	ctx := context.Background()
	if err := b.handler(ctx, channel.Update{
		ChannelType:      model.ChannelTelegram,
		PlatformUserID:   model.PlatformUserID(platformUserID),
		PlatformUpdateID: "tg:" + updateID,
		Input:            model.TextInput{Text: text},
		ChatID:           chatID,
	}); err != nil {
		b.logger.Error("telegram: error handling message",
			slog.String("user", platformUserID),
			slog.Any("error", err))
	}
	return nil
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

		ctx := context.Background()
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

	ctx := context.Background()
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

		ctx := context.Background()
		if err := b.handler(ctx, channel.Update{
			ChannelType:      model.ChannelTelegram,
			PlatformUserID:   model.PlatformUserID(platformUserID),
			PlatformUpdateID: "tg:" + updateID,
			Input:            model.CallbackInput{Data: data},
			ChatID:           chatID,
		}); err != nil {
			b.logger.Error("telegram: error handling callback",
				slog.String("user", platformUserID),
				slog.Any("error", err))
		}
		return nil
	})
}
