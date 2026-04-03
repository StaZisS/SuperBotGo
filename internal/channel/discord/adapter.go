package discord

import (
	"context"
	"fmt"
	"sync/atomic"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/filestore"
	"SuperBotGo/internal/model"

	"github.com/bwmarrin/discordgo"
)

var (
	_ channel.SilentSender  = (*Adapter)(nil)
	_ channel.StatusChecker = (*Adapter)(nil)
)

type Adapter struct {
	session   *discordgo.Session
	renderer  *Renderer
	connected *atomic.Bool
	fileStore filestore.FileStore
}

func NewAdapter(session *discordgo.Session, connected *atomic.Bool, fs filestore.FileStore) *Adapter {
	return &Adapter{
		session:   session,
		renderer:  NewRenderer(),
		connected: connected,
		fileStore: fs,
	}
}

func (a *Adapter) Connected() bool {
	return a.connected.Load()
}

func (a *Adapter) Type() model.ChannelType {
	return model.ChannelDiscord
}

func (a *Adapter) SendToUser(ctx context.Context, platformUserID model.PlatformUserID, msg model.Message) error {
	return a.sendToUser(ctx, platformUserID, msg, false)
}

func (a *Adapter) SendToChat(ctx context.Context, chatID string, msg model.Message) error {
	return a.sendMessage(ctx, chatID, msg, false)
}

func (a *Adapter) SendToUserSilent(ctx context.Context, platformUserID model.PlatformUserID, msg model.Message, silent bool) error {
	return a.sendToUser(ctx, platformUserID, msg, silent)
}

func (a *Adapter) SendToChatSilent(ctx context.Context, chatID string, msg model.Message, silent bool) error {
	return a.sendMessage(ctx, chatID, msg, silent)
}

func (a *Adapter) sendToUser(ctx context.Context, platformUserID model.PlatformUserID, msg model.Message, silent bool) error {
	dmChannel, err := a.session.UserChannelCreate(string(platformUserID))
	if err != nil {
		return fmt.Errorf("discord: create DM channel for user %s: %w", platformUserID, err)
	}
	return a.sendMessage(ctx, dmChannel.ID, msg, silent)
}

func (a *Adapter) sendMessage(_ context.Context, channelID string, msg model.Message, silent bool) error {
	if msg.IsEmpty() {
		return fmt.Errorf("discord: refusing to send empty message to channel %s", channelID)
	}

	rendered := a.renderer.Render(msg)

	msgSend := &discordgo.MessageSend{}

	if silent {
		msgSend.Flags = discordgo.MessageFlagsSuppressNotifications
	}

	if rendered.Text != "" {
		msgSend.Content = rendered.Text
	}

	if rendered.HasOptions {
		for _, row := range rendered.Buttons {
			actRow := discordgo.ActionsRow{}
			for _, btn := range row {
				actRow.Components = append(actRow.Components, discordgo.Button{
					Label:    btn.Label,
					Style:    discordgo.PrimaryButton,
					CustomID: btn.CustomID,
				})
			}
			msgSend.Components = append(msgSend.Components, actRow)
		}
	}

	for _, imageURL := range rendered.ImageURLs {
		msgSend.Embeds = append(msgSend.Embeds, &discordgo.MessageEmbed{
			Image: &discordgo.MessageEmbedImage{
				URL: imageURL,
			},
		})
	}

	if a.fileStore != nil {
		for _, ref := range rendered.FileRefs {
			reader, meta, fErr := a.fileStore.Get(context.Background(), ref.ID)
			if fErr != nil {
				return fmt.Errorf("discord: get file %q: %w", ref.ID, fErr)
			}
			name := ref.Name
			if name == "" && meta != nil {
				name = meta.Name
			}
			msgSend.Files = append(msgSend.Files, &discordgo.File{
				Name:   name,
				Reader: reader,
			})
			// Note: reader is closed by discordgo after sending.
		}
	}

	_, err := a.session.ChannelMessageSendComplex(channelID, msgSend)
	if err != nil {
		return fmt.Errorf("discord: send message to %s: %w", channelID, err)
	}
	return nil
}
