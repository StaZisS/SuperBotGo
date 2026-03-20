package discord

import (
	"context"
	"fmt"
	"log/slog"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/model"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	session *discordgo.Session
	handler channel.UpdateHandler
	logger  *slog.Logger
}

func NewBot(token string, handler channel.UpdateHandler, logger *slog.Logger) (*Bot, error) {
	if logger == nil {
		logger = slog.Default()
	}

	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("discord: create session: %w", err)
	}

	session.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent

	b := &Bot{
		session: session,
		handler: handler,
		logger:  logger,
	}

	b.registerHandlers()

	return b, nil
}

func (b *Bot) Adapter() *Adapter {
	return NewAdapter(b.session)
}

func (b *Bot) Start(ctx context.Context) error {
	b.logger.Info("Discord bot opening gateway connection")

	if err := b.session.Open(); err != nil {
		return fmt.Errorf("discord: open gateway: %w", err)
	}

	<-ctx.Done()
	b.logger.Info("Discord bot closing gateway connection")
	return b.session.Close()
}

func (b *Bot) Stop() error {
	return b.session.Close()
}

func (b *Bot) registerHandlers() {

	b.session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {

		if m.Author == nil || m.Author.Bot {
			return
		}
		text := m.Content
		if text == "" {
			return
		}

		platformUserID := m.Author.ID
		chatID := m.ChannelID

		b.logger.Info("discord: received message",
			slog.String("user", platformUserID),
			slog.String("channel", chatID),
			slog.String("text", text))

		ctx := context.Background()
		if err := b.handler.OnUpdate(ctx, model.ChannelDiscord, model.PlatformUserID(platformUserID), model.TextInput{Text: text}, chatID); err != nil {
			b.logger.Error("discord: error handling message",
				slog.String("user", platformUserID),
				slog.Any("error", err))
		}
	})

	b.session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type != discordgo.InteractionMessageComponent {
			return
		}

		data := i.MessageComponentData()
		platformUserID := ""
		if i.Member != nil && i.Member.User != nil {
			platformUserID = i.Member.User.ID
		} else if i.User != nil {
			platformUserID = i.User.ID
		}
		chatID := i.ChannelID

		b.logger.Info("discord: received button interaction",
			slog.String("user", platformUserID),
			slog.String("channel", chatID),
			slog.String("custom_id", data.CustomID))

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})

		ctx := context.Background()
		if err := b.handler.OnUpdate(ctx, model.ChannelDiscord, model.PlatformUserID(platformUserID), model.CallbackInput{Data: data.CustomID}, chatID); err != nil {
			b.logger.Error("discord: error handling button",
				slog.String("user", platformUserID),
				slog.Any("error", err))
		}
	})
}
