package discord

import (
	"context"
	"fmt"

	"SuperBotGo/internal/model"

	"github.com/bwmarrin/discordgo"
)

type Adapter struct {
	session  *discordgo.Session
	renderer *Renderer
}

func NewAdapter(session *discordgo.Session) *Adapter {
	return &Adapter{
		session:  session,
		renderer: NewRenderer(),
	}
}

func (a *Adapter) Type() model.ChannelType {
	return model.ChannelDiscord
}

func (a *Adapter) SendToUser(ctx context.Context, platformUserID model.PlatformUserID, msg model.Message) error {
	dmChannel, err := a.session.UserChannelCreate(string(platformUserID))
	if err != nil {
		return fmt.Errorf("discord: create DM channel for user %s: %w", platformUserID, err)
	}
	return a.sendMessage(ctx, dmChannel.ID, msg)
}

func (a *Adapter) SendToChat(ctx context.Context, chatID string, msg model.Message) error {
	return a.sendMessage(ctx, chatID, msg)
}

func (a *Adapter) sendMessage(_ context.Context, channelID string, msg model.Message) error {
	rendered := a.renderer.Render(msg)

	msgSend := &discordgo.MessageSend{}

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

	_, err := a.session.ChannelMessageSendComplex(channelID, msgSend)
	if err != nil {
		return fmt.Errorf("discord: send message to %s: %w", channelID, err)
	}
	return nil
}
