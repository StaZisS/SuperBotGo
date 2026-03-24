package core

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/notification"
	"SuperBotGo/internal/state"
)

var localeNames = map[string]string{
	"en": "English",
	"ru": "Русский",
}

type UserLocaleUpdater interface {
	UpdateLocale(ctx context.Context, userID model.GlobalUserID, locale string) error
}

func SettingsCommand() *state.CommandDefinition {
	return state.NewCommand("settings").
		Description("User Settings").
		Step("action", func(s *state.StepBuilder) {
			s.Prompt(func(p *state.PromptBuilder) {
				p.LocalizedText("settings.title", model.StyleHeader)
				p.LocalizedOptions("settings.action_prompt", func(o *state.OptionsBuilder) {
					o.LocalizedOption("settings.change_language", "change_language")
					o.LocalizedOption("settings.notification_channel", "notification_channel")
					o.LocalizedOption("settings.mute_mentions", "mute_mentions")
					o.LocalizedOption("settings.work_hours", "work_hours")
				})
			})
		}).
		Branch("action", func(b *state.BranchBuilder) {
			b.Case("change_language", func(n *state.NodeListBuilder) {
				n.Step("language", func(s *state.StepBuilder) {
					s.Prompt(func(p *state.PromptBuilder) {
						p.LocalizedText("settings.select_language", model.StylePlain)
						p.LocalizedOptions("settings.language_prompt", func(o *state.OptionsBuilder) {
							o.Add("English", "en")
							o.Add("Русский", "ru")
						})
					})
				})
			})
			b.Case("notification_channel", func(n *state.NodeListBuilder) {
				n.Step("preferred_channel", func(s *state.StepBuilder) {
					s.Prompt(func(p *state.PromptBuilder) {
						p.LocalizedText("settings.select_channel", model.StylePlain)
						p.LocalizedOptions("settings.channel_prompt", func(o *state.OptionsBuilder) {
							o.Add("Telegram", "TELEGRAM")
							o.Add("Discord", "DISCORD")
							o.Add("Telegram → Discord", "TELEGRAM,DISCORD")
							o.Add("Discord → Telegram", "DISCORD,TELEGRAM")
						})
					})
				})
			})
			b.Case("mute_mentions", func(n *state.NodeListBuilder) {
				n.Step("mute_value", func(s *state.StepBuilder) {
					s.Prompt(func(p *state.PromptBuilder) {
						p.LocalizedText("settings.mute_mentions_prompt", model.StylePlain)
						p.LocalizedOptions("settings.mute_mentions_options", func(o *state.OptionsBuilder) {
							o.LocalizedOption("settings.mute_on", "true")
							o.LocalizedOption("settings.mute_off", "false")
						})
					})
				})
			})
			b.Case("work_hours", func(n *state.NodeListBuilder) {
				n.Step("work_hours_value", func(s *state.StepBuilder) {
					s.Prompt(func(p *state.PromptBuilder) {
						p.LocalizedText("settings.work_hours_prompt", model.StylePlain)
						p.LocalizedOptions("settings.work_hours_options", func(o *state.OptionsBuilder) {
							o.LocalizedOption("settings.wh_9_18", "9-18")
							o.LocalizedOption("settings.wh_8_17", "8-17")
							o.LocalizedOption("settings.wh_10_19", "10-19")
							o.LocalizedOption("settings.wh_disable", "off")
						})
					})
				})
				n.Step("timezone", func(s *state.StepBuilder) {
					s.Prompt(func(p *state.PromptBuilder) {
						p.LocalizedText("settings.timezone_prompt", model.StylePlain)
						p.LocalizedOptions("settings.timezone_options", func(o *state.OptionsBuilder) {
							o.Add("Europe/Moscow (UTC+3)", "Europe/Moscow")
							o.Add("Europe/London (UTC+0)", "Europe/London")
							o.Add("Asia/Novosibirsk (UTC+7)", "Asia/Novosibirsk")
							o.Add("Asia/Vladivostok (UTC+10)", "Asia/Vladivostok")
						})
					})
				})
			})
		}).
		Build()
}

func (p *Plugin) handleSettings(ctx context.Context, m *model.MessengerTriggerData) error {
	switch m.Params.Get("action") {
	case "change_language":
		return p.changeLanguage(ctx, m)
	case "notification_channel":
		return p.changeNotificationChannel(ctx, m)
	case "mute_mentions":
		return p.toggleMuteMentions(ctx, m)
	case "work_hours":
		return p.setWorkHours(ctx, m)
	default:
		return p.api.Reply(ctx, m,
			model.NewTextMessage(i18n.Get("settings.unknown_action", m.Locale)))
	}
}

func (p *Plugin) changeLanguage(ctx context.Context, m *model.MessengerTriggerData) error {
	newLocale := m.Params.Get("language")
	if newLocale == "" {
		return nil
	}

	if err := p.userService.UpdateLocale(ctx, m.UserID, newLocale); err != nil {
		return err
	}

	displayName := localeNames[newLocale]
	if displayName == "" {
		displayName = newLocale
	}

	return p.api.Reply(ctx, m, model.Message{
		Blocks: []model.ContentBlock{
			model.TextBlock{
				Text:  i18n.Get("settings.language_updated", newLocale, displayName),
				Style: model.StyleHeader,
			},
		},
	})
}

func (p *Plugin) changeNotificationChannel(ctx context.Context, m *model.MessengerTriggerData) error {
	value := m.Params.Get("preferred_channel")
	if value == "" {
		return nil
	}

	prefs, err := p.getOrCreatePrefs(ctx, m.UserID)
	if err != nil {
		return err
	}

	prefs.ChannelPriority = notification.UnmarshalChannelPriority(value)
	if err := p.prefsRepo.SavePrefs(ctx, prefs); err != nil {
		return err
	}

	return p.api.Reply(ctx, m, model.Message{
		Blocks: []model.ContentBlock{
			model.TextBlock{
				Text:  i18n.Get("settings.channel_updated", m.Locale, value),
				Style: model.StyleHeader,
			},
		},
	})
}

func (p *Plugin) toggleMuteMentions(ctx context.Context, m *model.MessengerTriggerData) error {
	value := m.Params.Get("mute_value")
	if value == "" {
		return nil
	}

	prefs, err := p.getOrCreatePrefs(ctx, m.UserID)
	if err != nil {
		return err
	}

	prefs.MuteMentions = value == "true"
	if err := p.prefsRepo.SavePrefs(ctx, prefs); err != nil {
		return err
	}

	key := "settings.mentions_enabled"
	if prefs.MuteMentions {
		key = "settings.mentions_muted"
	}

	return p.api.Reply(ctx, m, model.Message{
		Blocks: []model.ContentBlock{
			model.TextBlock{
				Text:  i18n.Get(key, m.Locale),
				Style: model.StyleHeader,
			},
		},
	})
}

func (p *Plugin) setWorkHours(ctx context.Context, m *model.MessengerTriggerData) error {
	hoursValue := m.Params.Get("work_hours_value")
	if hoursValue == "" {
		return nil
	}

	prefs, err := p.getOrCreatePrefs(ctx, m.UserID)
	if err != nil {
		return err
	}

	if hoursValue == "off" {
		prefs.WorkHoursStart = nil
		prefs.WorkHoursEnd = nil
	} else {
		parts := strings.SplitN(hoursValue, "-", 2)
		if len(parts) == 2 {
			start, err1 := strconv.Atoi(parts[0])
			end, err2 := strconv.Atoi(parts[1])
			if err1 == nil && err2 == nil {
				prefs.WorkHoursStart = &start
				prefs.WorkHoursEnd = &end
			}
		}
	}

	tz := m.Params.Get("timezone")
	if tz != "" {
		prefs.Timezone = tz
	}

	if err := p.prefsRepo.SavePrefs(ctx, prefs); err != nil {
		return err
	}

	var replyText string
	if prefs.WorkHoursStart == nil {
		replyText = i18n.Get("settings.work_hours_disabled", m.Locale)
	} else {
		replyText = i18n.Get("settings.work_hours_set", m.Locale,
			fmt.Sprintf("%d:00", *prefs.WorkHoursStart),
			fmt.Sprintf("%d:00", *prefs.WorkHoursEnd),
			prefs.Timezone)
	}

	return p.api.Reply(ctx, m, model.Message{
		Blocks: []model.ContentBlock{
			model.TextBlock{Text: replyText, Style: model.StyleHeader},
		},
	})
}

func (p *Plugin) getOrCreatePrefs(ctx context.Context, userID model.GlobalUserID) (*model.NotificationPrefs, error) {
	prefs, err := p.prefsRepo.GetPrefs(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("settings: get notification prefs: %w", err)
	}
	if prefs == nil {
		prefs = &model.NotificationPrefs{
			GlobalUserID: userID,
			Timezone:     "UTC",
		}
	}
	return prefs, nil
}
