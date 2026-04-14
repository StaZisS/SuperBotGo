package config

import "testing"

func TestValidateVKCallbackRequiresURL(t *testing.T) {
	cfg := validConfig()
	cfg.VK.Mode = "callback"

	err := cfg.Validate()
	if err == nil || err.Error() != "vk.callback_url is required when vk.mode=callback" {
		t.Fatalf("Validate() error = %v, want vk.callback_url validation error", err)
	}
}

func TestValidateRejectsInvalidCallbackPaths(t *testing.T) {
	cfg := validConfig()
	cfg.VK.CallbackPath = "vk/callback"

	err := cfg.Validate()
	if err == nil || err.Error() != "vk.callback_path must start with \"/\", got \"vk/callback\"" {
		t.Fatalf("Validate() error = %v, want vk.callback_path validation error", err)
	}

	cfg = validConfig()
	cfg.Mattermost.ActionsPath = "mattermost/actions"

	err = cfg.Validate()
	if err == nil || err.Error() != "mattermost.actions_path must start with \"/\", got \"mattermost/actions\"" {
		t.Fatalf("Validate() error = %v, want mattermost.actions_path validation error", err)
	}

	cfg = validConfig()
	cfg.Mattermost.CommandPath = "mattermost/command"

	err = cfg.Validate()
	if err == nil || err.Error() != "mattermost.command_path must start with \"/\", got \"mattermost/command\"" {
		t.Fatalf("Validate() error = %v, want mattermost.command_path validation error", err)
	}
}

func TestValidateMattermostActionsRequireSecret(t *testing.T) {
	cfg := validConfig()
	cfg.Mattermost.ActionsURL = "https://bot.example.com/mattermost/actions"

	err := cfg.Validate()
	if err == nil || err.Error() != "mattermost.actions_url and mattermost.actions_secret must be set together" {
		t.Fatalf("Validate() error = %v, want mattermost.actions validation error", err)
	}
}

func TestValidateAcceptsInteractiveChannelConfig(t *testing.T) {
	cfg := validConfig()
	cfg.VK.Mode = "callback"
	cfg.VK.CallbackURL = "https://bot.example.com/vk/callback"
	cfg.VK.CallbackPath = "/vk/callback"
	cfg.Mattermost.URL = "https://mattermost.example.com"
	cfg.Mattermost.Token = "token"
	cfg.Mattermost.ActionsURL = "https://bot.example.com/mattermost/actions"
	cfg.Mattermost.ActionsPath = "/mattermost/actions"
	cfg.Mattermost.ActionsSecret = "secret"
	cfg.Mattermost.CommandURL = "https://bot.example.com/mattermost/command"
	cfg.Mattermost.CommandPath = "/mattermost/command"
	cfg.Mattermost.CommandTrigger = "hits"
	cfg.Mattermost.CommandToken = "token"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestValidateRejectsInvalidMattermostCommandTrigger(t *testing.T) {
	cfg := validConfig()
	cfg.Mattermost.CommandTrigger = "/hits"

	err := cfg.Validate()
	if err == nil || err.Error() != "mattermost.command_trigger must not start with \"/\", got \"/hits\"" {
		t.Fatalf("Validate() error = %v, want command_trigger leading slash validation error", err)
	}

	cfg = validConfig()
	cfg.Mattermost.CommandTrigger = "hits bot"

	err = cfg.Validate()
	if err == nil || err.Error() != "mattermost.command_trigger must not contain whitespace, got \"hits bot\"" {
		t.Fatalf("Validate() error = %v, want command_trigger whitespace validation error", err)
	}
}

func validConfig() Config {
	return Config{
		Telegram: TelegramConfig{
			Mode: "polling",
		},
		Discord: DiscordConfig{
			ShardCount: 1,
		},
		VK: VKConfig{
			Mode:         "longpoll",
			CallbackPath: "/vk/callback",
		},
		Mattermost: MattermostConfig{
			ActionsPath:    "/mattermost/actions",
			CommandPath:    "/mattermost/command",
			CommandTrigger: "hits",
		},
	}
}
