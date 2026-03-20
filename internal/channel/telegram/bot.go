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

// Bot wraps a Telegram bot instance and routes events to an UpdateHandler.
type Bot struct {
	bot     *tele.Bot
	handler channel.UpdateHandler
	logger  *slog.Logger
}

// NewBot creates a Telegram bot. The bot is not started until Start is called.
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

// Adapter returns the ChannelAdapter for this bot.
func (b *Bot) Adapter() *Adapter {
	return NewAdapter(b.bot)
}

// Start begins long-polling. It blocks until the context is cancelled.
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

// Stop halts the bot polling.
func (b *Bot) Stop() {
	b.bot.Stop()
}

// RegisterCommands registers Telegram handlers for each known bot command.
// Must be called before Start().
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
			if idx := indexOf(data[1:], '\f'); idx >= 0 {
				data = data[idx+2:]
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
