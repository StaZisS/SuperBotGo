package core

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

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
		LocalizedDescription(map[string]string{
			"en": "User settings",
			"ru": "Настройки пользователя",
		}).
		Description("User Settings").
		Step("action", func(s *state.StepBuilder) {
			s.Prompt(func(p *state.PromptBuilder) {
				p.LocalizedText("settings.title", model.StyleHeader)
				p.LocalizedOptions("settings.action_prompt", func(o *state.OptionsBuilder) {
					o.LocalizedOption("settings.view_current", "view_current")
					o.LocalizedOption("settings.change_language", "change_language")
					o.LocalizedOption("settings.notification_channel", "notification_channel")
					o.LocalizedOption("settings.mute_mentions", "mute_mentions")
					o.LocalizedOption("settings.work_hours", "work_hours")
				})
			})
		}).
		Branch("action", func(b *state.BranchBuilder) {
			b.Case("view_current", func(n *state.NodeListBuilder) {})
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
							o.LocalizedOption("settings.ch_telegram", "TELEGRAM")
							o.LocalizedOption("settings.ch_discord", "DISCORD")
							o.LocalizedOption("settings.ch_vk", "VK")
							o.LocalizedOption("settings.ch_mattermost", "MATTERMOST")
							o.LocalizedOption("settings.ch_tg_then_dc", "TELEGRAM,DISCORD")
							o.LocalizedOption("settings.ch_dc_then_tg", "DISCORD,TELEGRAM")
						})
					})
				})
			})
			b.Case("mute_mentions", func(n *state.NodeListBuilder) {
				n.Step("mute_value", func(s *state.StepBuilder) {
					s.Prompt(func(p *state.PromptBuilder) {
						p.LocalizedText("settings.mute_mentions_prompt", model.StylePlain)
						p.LocalizedOptions("settings.mute_mentions_options", func(o *state.OptionsBuilder) {
							o.LocalizedOption("settings.mute_off", "false")
							o.LocalizedOption("settings.mute_on", "true")
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
							o.LocalizedOption("settings.wh_custom", "custom")
							o.LocalizedOption("settings.wh_disable", "off")
						})
					})
				})
				n.Step("custom_hours", func(s *state.StepBuilder) {
					s.VisibleWhen(func(params model.OptionMap) bool {
						return params.Get("work_hours_value") == "custom"
					})
					s.Validate(func(input model.UserInput) bool {
						return validateHoursRange(input.TextValue())
					})
					s.Prompt(func(p *state.PromptBuilder) {
						p.LocalizedText("settings.custom_hours_prompt", model.StylePlain)
					})
				})
				n.Step("timezone", func(s *state.StepBuilder) {
					s.VisibleWhen(func(params model.OptionMap) bool {
						return params.Get("work_hours_value") != "off"
					})
					s.Prompt(func(p *state.PromptBuilder) {
						p.LocalizedText("settings.timezone_prompt", model.StylePlain)
						p.LocalizedOptions("settings.timezone_options", func(o *state.OptionsBuilder) {
							o.Add("Europe/Moscow (UTC+3)", "Europe/Moscow")
							o.Add("Europe/Kaliningrad (UTC+2)", "Europe/Kaliningrad")
							o.Add("Europe/Samara (UTC+4)", "Europe/Samara")
							o.Add("Asia/Yekaterinburg (UTC+5)", "Asia/Yekaterinburg")
							o.Add("Asia/Novosibirsk (UTC+7)", "Asia/Novosibirsk")
							o.Add("Asia/Vladivostok (UTC+10)", "Asia/Vladivostok")
							o.LocalizedOption("settings.tz_other", "custom")
						})
					})
				})
				n.Step("custom_timezone", func(s *state.StepBuilder) {
					s.VisibleWhen(func(params model.OptionMap) bool {
						return params.Get("timezone") == "custom"
					})
					s.Validate(func(input model.UserInput) bool {
						_, err := time.LoadLocation(input.TextValue())
						return err == nil
					})
					s.Prompt(func(p *state.PromptBuilder) {
						p.LocalizedText("settings.custom_tz_prompt", model.StylePlain)
					})
				})
			})
		}).
		Build()
}

func validateHoursRange(s string) bool {
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return false
	}
	start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	return err1 == nil && err2 == nil && start >= 0 && start <= 23 && end >= 0 && end <= 23 && start != end
}

func (p *Plugin) handleSettings(ctx context.Context, m *model.MessengerTriggerData) error {
	switch m.Params.Get("action") {
	case "view_current":
		return p.viewSettings(ctx, m)
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

func (p *Plugin) viewSettings(ctx context.Context, m *model.MessengerTriggerData) error {
	prefs, err := p.getOrCreatePrefs(ctx, m.UserID)
	if err != nil {
		return err
	}

	locale := m.Locale
	langName := localeNames[locale]
	if langName == "" {
		langName = locale
	}

	channel := formatChannelDisplay(strings.Join(channelTypesToStrings(prefs.ChannelPriority), ","))
	if channel == "" {
		channel = i18n.Get("settings.view_not_set", locale)
	}

	mentionsKey := "settings.view_mentions_on"
	if prefs.MuteMentions {
		mentionsKey = "settings.view_mentions_off"
	}

	var workHours string
	if prefs.WorkHoursStart != nil && prefs.WorkHoursEnd != nil {
		workHours = fmt.Sprintf("%d:00 – %d:00 (%s)", *prefs.WorkHoursStart, *prefs.WorkHoursEnd, prefs.Timezone)
	} else {
		workHours = i18n.Get("settings.view_wh_off", locale)
	}

	return p.api.Reply(ctx, m, model.Message{
		Blocks: []model.ContentBlock{
			model.TextBlock{
				Text:  i18n.Get("settings.view_title", locale),
				Style: model.StyleHeader,
			},
			model.TextBlock{
				Text: i18n.Get("settings.view_language", locale, langName) + "\n" +
					i18n.Get("settings.view_channel", locale, channel) + "\n" +
					i18n.Get("settings.view_mentions", locale, i18n.Get(mentionsKey, locale)) + "\n" +
					i18n.Get("settings.view_work_hours", locale, workHours),
				Style: model.StylePlain,
			},
		},
	})
}

func channelTypesToStrings(types []model.ChannelType) []string {
	result := make([]string, len(types))
	for i, t := range types {
		result[i] = string(t)
	}
	return result
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
				Text:  i18n.Get("settings.channel_updated", m.Locale, formatChannelDisplay(value)),
				Style: model.StyleHeader,
			},
		},
	})
}

func formatChannelDisplay(raw string) string {
	parts := strings.Split(raw, ",")
	for i, p := range parts {
		switch strings.TrimSpace(p) {
		case "TELEGRAM":
			parts[i] = "Telegram"
		case "DISCORD":
			parts[i] = "Discord"
		case "VK":
			parts[i] = "VK"
		case "MATTERMOST":
			parts[i] = "Mattermost"
		}
	}
	return strings.Join(parts, " → ")
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
		if hoursValue == "custom" {
			hoursValue = m.Params.Get("custom_hours")
		}
		parts := strings.SplitN(hoursValue, "-", 2)
		if len(parts) == 2 {
			start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
			end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err1 == nil && err2 == nil {
				prefs.WorkHoursStart = &start
				prefs.WorkHoursEnd = &end
			}
		}
	}

	tz := m.Params.Get("timezone")
	if tz == "custom" {
		tz = m.Params.Get("custom_timezone")
	}
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
