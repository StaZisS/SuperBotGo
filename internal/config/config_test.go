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

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
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
			ActionsPath: "/mattermost/actions",
		},
	}
}
