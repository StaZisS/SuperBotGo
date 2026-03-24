package settings

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/notification"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/state"
)

var localeNames = map[string]string{
	"en": "English",
	"ru": "Русский",
}

type UserLocaleUpdater interface {
	UpdateLocale(ctx context.Context, userID model.GlobalUserID, locale string) error
}

type Plugin struct {
	api         *plugin.SenderAPI
	userService UserLocaleUpdater
	prefsRepo   notification.PrefsRepository
	cmdDef      *state.CommandDefinition
}

func New(api *plugin.SenderAPI, userService UserLocaleUpdater, prefsRepo notification.PrefsRepository) *Plugin {
	return &Plugin{
		api:         api,
		userService: userService,
		prefsRepo:   prefsRepo,
		cmdDef:      SettingsCommand(),
	}
}

func (p *Plugin) ID() string                           { return "settings" }
func (p *Plugin) Name() string                         { return "User Settings" }
func (p *Plugin) Version() string                      { return "1.0.0" }
func (p *Plugin) SupportedRoles() []string             { return []string{"USER", "ADMIN"} }
func (p *Plugin) Commands() []*state.CommandDefinition { return []*state.CommandDefinition{p.cmdDef} }

func (p *Plugin) HandleEvent(ctx context.Context, event model.Event) (*model.EventResponse, error) {
	m, err := event.Messenger()
	if err != nil {
		return nil, fmt.Errorf("settings: parse messenger data: %w", err)
	}

	switch m.Params.Get("action") {
	case "change_language":
		return nil, p.changeLanguage(ctx, m)
	case "notification_channel":
		return nil, p.changeNotificationChannel(ctx, m)
	case "mute_mentions":
		return nil, p.toggleMuteMentions(ctx, m)
	case "work_hours":
		return nil, p.setWorkHours(ctx, m)
	default:
		return nil, p.api.Reply(ctx, m,
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
