package discord

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/model"

	"github.com/bwmarrin/discordgo"
)

type BotConfig struct {
	Token      string
	ShardID    int
	ShardCount int // 0 or 1 = no sharding
}

type Bot struct {
	session     *discordgo.Session
	handler     channel.UpdateHandlerFunc
	joinHandler channel.ChatJoinHandler
	logger      *slog.Logger
	connected   atomic.Bool
}

func NewBot(cfg BotConfig, handler channel.UpdateHandlerFunc, joinHandler channel.ChatJoinHandler, logger *slog.Logger) (*Bot, error) {
	if logger == nil {
		logger = slog.Default()
	}

	session, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("discord: create session: %w", err)
	}

	if cfg.ShardCount > 1 {
		session.ShardID = cfg.ShardID
		session.ShardCount = cfg.ShardCount
		logger.Info("discord: sharding enabled",
			slog.Int("shard_id", cfg.ShardID),
			slog.Int("shard_count", cfg.ShardCount))
	}

	session.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent

	b := &Bot{
		session:     session,
		handler:     handler,
		joinHandler: joinHandler,
		logger:      logger,
	}

	b.registerHandlers()

	return b, nil
}

func (b *Bot) Adapter() *Adapter {
	return NewAdapter(b.session, &b.connected)
}

func (b *Bot) Start(ctx context.Context) error {
	b.logger.Info("Discord bot opening gateway connection")

	if err := b.session.Open(); err != nil {
		return fmt.Errorf("discord: open gateway: %w", err)
	}
	b.connected.Store(true)

	<-ctx.Done()
	b.connected.Store(false)
	b.logger.Info("Discord bot closing gateway connection")
	return b.session.Close()
}

func (b *Bot) Stop() error {
	return b.session.Close()
}

func (b *Bot) registerHandlers() {

	b.session.AddHandler(func(s *discordgo.Session, g *discordgo.GuildCreate) {
		if b.joinHandler == nil || g.Guild == nil {
			return
		}

		b.logger.Info("discord: bot joined guild",
			slog.String("guild_id", g.ID),
			slog.String("guild_name", g.Name))

		ctx := context.Background()

		for _, ch := range g.Channels {
			if ch.Type != discordgo.ChannelTypeGuildText {
				continue
			}

			b.logger.Info("discord: registering channel",
				slog.String("channel_id", ch.ID),
				slog.String("channel_name", ch.Name),
				slog.String("guild_name", g.Name))

			title := g.Name + " / " + ch.Name
			if err := b.joinHandler.OnChatJoin(ctx, model.ChannelDiscord, ch.ID, model.ChatKindGroup, title); err != nil {
				b.logger.Error("discord: failed to register channel on guild join",
					slog.String("channel_id", ch.ID),
					slog.Any("error", err))
			}
		}
	})

	b.session.AddHandler(func(s *discordgo.Session, g *discordgo.GuildDelete) {
		if b.joinHandler == nil || g.Guild == nil {
			return
		}

		b.logger.Info("discord: bot removed from guild",
			slog.String("guild_id", g.ID))

		ctx := context.Background()

		var channels []*discordgo.Channel
		if g.BeforeDelete != nil {
			channels = g.BeforeDelete.Channels
		}

		if len(channels) == 0 {
			b.logger.Warn("discord: guild channels not available in cache, cannot unregister",
				slog.String("guild_id", g.ID))
			return
		}

		for _, ch := range channels {
			if ch.Type != discordgo.ChannelTypeGuildText {
				continue
			}
			if err := b.joinHandler.OnChatLeave(ctx, model.ChannelDiscord, ch.ID); err != nil {
				b.logger.Error("discord: failed to unregister channel on guild leave",
					slog.String("channel_id", ch.ID),
					slog.Any("error", err))
			}
		}
	})

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
		if err := b.handler(ctx, channel.Update{
			ChannelType:      model.ChannelDiscord,
			PlatformUserID:   model.PlatformUserID(platformUserID),
			PlatformUpdateID: "dc:msg:" + m.ID,
			Input:            model.TextInput{Text: text},
			ChatID:           chatID,
			Username:         m.Author.Username,
		}); err != nil {
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
		discordUsername := ""
		if i.Member != nil && i.Member.User != nil {
			discordUsername = i.Member.User.Username
		} else if i.User != nil {
			discordUsername = i.User.Username
		}
		if err := b.handler(ctx, channel.Update{
			ChannelType:      model.ChannelDiscord,
			PlatformUserID:   model.PlatformUserID(platformUserID),
			PlatformUpdateID: "dc:int:" + i.ID,
			Input:            model.CallbackInput{Data: data.CustomID},
			ChatID:           chatID,
			Username:         discordUsername,
		}); err != nil {
			b.logger.Error("discord: error handling button",
				slog.String("user", platformUserID),
				slog.Any("error", err))
		}
	})
}
