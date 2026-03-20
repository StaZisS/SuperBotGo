package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/model"

	tele "gopkg.in/telebot.v3"
)

type Bot struct {
	bot     *tele.Bot
	handler channel.UpdateHandler
	logger  *slog.Logger
}

func NewBot(token string, handler channel.UpdateHandler, logger *slog.Logger) (*Bot, error) {
	if logger == nil {
		logger = slog.Default()
	}

	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		return nil, fmt.Errorf("telegram: create bot: %w", err)
	}

	tb := &Bot{
		bot:     b,
		handler: handler,
		logger:  logger,
	}

	tb.registerHandlers()

	return tb, nil
}

func (b *Bot) Adapter() *Adapter {
	return NewAdapter(b.bot)
}

func (b *Bot) Start(ctx context.Context) error {
	b.logger.Info("Telegram bot starting long polling")

	go func() {
		<-ctx.Done()
		b.logger.Info("Telegram bot stopping")
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
	text := c.Text()

	b.logger.Info("telegram: received message",
		slog.String("user", platformUserID),
		slog.String("chat", chatID),
		slog.String("text", text))

	ctx := context.Background()
	if err := b.handler.OnUpdate(ctx, model.ChannelTelegram, model.PlatformUserID(platformUserID), model.TextInput{Text: text}, chatID); err != nil {
		b.logger.Error("telegram: error handling message",
			slog.String("user", platformUserID),
			slog.Any("error", err))
	}
	return nil
}

func (b *Bot) registerHandlers() {

	b.bot.Handle(tele.OnText, func(c tele.Context) error {
		return b.handleTextMessage(c)
	})

	b.bot.Handle(tele.OnCallback, func(c tele.Context) error {
		_ = c.Respond()

		chatID := strconv.FormatInt(c.Chat().ID, 10)
		platformUserID := strconv.FormatInt(c.Sender().ID, 10)
		data := c.Callback().Data

		if len(data) > 0 && data[0] == '\f' {
			rest := data[1:]
			if idx := indexOf(rest, '|'); idx >= 0 {
				data = rest[idx+1:]
			} else if idx := indexOf(rest, '\f'); idx >= 0 {
				data = rest[idx+1:]
			} else {
				data = rest
			}
		}

		b.logger.Info("telegram: received callback",
			slog.String("user", platformUserID),
			slog.String("chat", chatID),
			slog.String("data", data))

		ctx := context.Background()
		if err := b.handler.OnUpdate(ctx, model.ChannelTelegram, model.PlatformUserID(platformUserID), model.CallbackInput{Data: data}, chatID); err != nil {
			b.logger.Error("telegram: error handling callback",
				slog.String("user", platformUserID),
				slog.Any("error", err))
		}
		return nil
	})
}

func indexOf(s string, ch byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ch {
			return i
		}
	}
	return -1
}
