package discord

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/filestore"
	"SuperBotGo/internal/model"

	"github.com/bwmarrin/discordgo"
)

type BotConfig struct {
	Token      string
	ShardID    int
	ShardCount int // 0 or 1 = no sharding
}

type Bot struct {
	session      *discordgo.Session
	handler      channel.UpdateHandlerFunc
	joinHandler  channel.ChatJoinHandler
	fileStore    filestore.FileStore
	httpClient   *http.Client
	maxFileSize  int64
	logger       *slog.Logger
	connected    atomic.Bool
	lifecycleCtx context.Context
}

func NewBot(cfg BotConfig, handler channel.UpdateHandlerFunc, joinHandler channel.ChatJoinHandler, fs filestore.FileStore, maxFileSize int64, logger *slog.Logger) (*Bot, error) {
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
		fileStore:   fs,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		maxFileSize: maxFileSize,
		logger:      logger,
	}

	b.registerHandlers()

	return b, nil
}

func (b *Bot) Adapter() *Adapter {
	return NewAdapter(b.session, &b.connected, b.fileStore)
}

func (b *Bot) Start(ctx context.Context) error {
	b.logger.Info("Discord bot opening gateway connection")
	b.lifecycleCtx = ctx

	if err := b.session.Open(); err != nil {
		return fmt.Errorf("discord: open gateway: %w", err)
	}
	b.connected.Store(true)

	<-ctx.Done()
	b.connected.Store(false)
	b.logger.Info("Discord bot closing gateway connection")
	return b.session.Close()
}

// deriveContext returns a context derived from the bot lifecycle context.
func (b *Bot) deriveContext() context.Context {
	if b.lifecycleCtx != nil {
		return b.lifecycleCtx
	}
	return context.Background()
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

		ctx := b.deriveContext()

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

		ctx := b.deriveContext()

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
		platformUserID := m.Author.ID
		chatID := m.ChannelID

		if len(m.Attachments) > 0 && b.fileStore != nil {
			var refs []model.FileRef
			for _, att := range m.Attachments {
				if b.maxFileSize > 0 && int64(att.Size) > b.maxFileSize {
					b.logger.Warn("discord: attachment too large, skipping",
						slog.String("user", platformUserID),
						slog.Int("size", att.Size),
						slog.Int64("max_size", b.maxFileSize))
					continue
				}

				ref, err := b.downloadAndStore(att)
				if err != nil {
					b.logger.Error("discord: failed to download attachment",
						slog.String("user", platformUserID),
						slog.String("url", att.URL),
						slog.Any("error", err))
					continue
				}
				refs = append(refs, ref)
			}

			if len(refs) > 0 {
				b.logger.Info("discord: received files",
					slog.String("user", platformUserID),
					slog.String("channel", chatID),
					slog.Int("count", len(refs)))

				ctx := b.deriveContext()
				if err := b.handler(ctx, channel.Update{
					ChannelType:      model.ChannelDiscord,
					PlatformUserID:   model.PlatformUserID(platformUserID),
					PlatformUpdateID: "dc:msg:" + m.ID,
					Input:            model.FileInput{Caption: text, Files: refs},
					ChatID:           chatID,
					Username:         m.Author.Username,
				}); err != nil {
					b.logger.Error("discord: error handling file message",
						slog.String("user", platformUserID),
						slog.Any("error", err))
				}
				return // already handled as file message
			}
		}

		if text == "" {
			return
		}

		b.logger.Info("discord: received message",
			slog.String("user", platformUserID),
			slog.String("channel", chatID),
			slog.String("text", text))

		ctx := b.deriveContext()
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
		platformUserID, discordUsername := extractInteractionUser(i)
		chatID := i.ChannelID

		b.logger.Info("discord: received button interaction",
			slog.String("user", platformUserID),
			slog.String("channel", chatID),
			slog.String("custom_id", data.CustomID))

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})

		ctx := b.deriveContext()
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

func (b *Bot) downloadAndStore(att *discordgo.MessageAttachment) (model.FileRef, error) {
	resp, err := b.httpClient.Get(att.URL)
	if err != nil {
		return model.FileRef{}, fmt.Errorf("download %q: %w", att.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return model.FileRef{}, fmt.Errorf("download %q: status %d", att.URL, resp.StatusCode)
	}

	fileType := detectFileType(att.ContentType)

	ctx := b.deriveContext()
	return b.fileStore.Store(ctx, filestore.FileMeta{
		Name:     att.Filename,
		MIMEType: att.ContentType,
		Size:     int64(att.Size),
		FileType: fileType,
	}, resp.Body)
}

func detectFileType(contentType string) model.FileType {
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return model.FileTypePhoto
	case strings.HasPrefix(contentType, "audio/"):
		return model.FileTypeAudio
	case strings.HasPrefix(contentType, "video/"):
		return model.FileTypeVideo
	default:
		return model.FileTypeDocument
	}
}

// extractInteractionUser returns the user ID and username from a Discord interaction,
// handling both guild (Member) and DM (User) contexts.
func extractInteractionUser(i *discordgo.InteractionCreate) (id, username string) {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID, i.Member.User.Username
	}
	if i.User != nil {
		return i.User.ID, i.User.Username
	}
	return "", ""
}
