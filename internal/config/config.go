package config

import (
	"fmt"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	DefaultLocale string         `koanf:"default_locale"`
	Database      DatabaseConfig `koanf:"database"`
	Redis         RedisConfig    `koanf:"redis"`
	Telegram      TelegramConfig `koanf:"telegram"`
	Discord       DiscordConfig  `koanf:"discord"`
	Admin         AdminConfig    `koanf:"admin"`
}

type AdminConfig struct {
	Port       int      `koanf:"port"`
	ModulesDir string   `koanf:"modules_dir"`
	BlobStore  string   `koanf:"blob_store"`
	APIKey     string   `koanf:"api_key"`
	S3         S3Config `koanf:"s3"`
}

type S3Config struct {
	Bucket    string `koanf:"bucket"`
	Region    string `koanf:"region"`
	Endpoint  string `koanf:"endpoint"`
	AccessKey string `koanf:"access_key"`
	SecretKey string `koanf:"secret_key"`
	Prefix    string `koanf:"prefix"`
}

type DatabaseConfig struct {
	Host     string `koanf:"host"`
	Port     int    `koanf:"port"`
	User     string `koanf:"user"`
	Password string `koanf:"password"`
	DBName   string `koanf:"dbname"`
	SSLMode  string `koanf:"sslmode"`
}

type RedisConfig struct {
	Addr     string `koanf:"addr"`
	Password string `koanf:"password"`
	DB       int    `koanf:"db"`
}

type TelegramConfig struct {
	Token         string `koanf:"token"`
	Mode          string `koanf:"mode"`           // "polling" (default) or "webhook"
	WebhookURL    string `koanf:"webhook_url"`    // public HTTPS URL for webhook mode
	WebhookSecret string `koanf:"webhook_secret"` // secret token for webhook validation
	WebhookListen string `koanf:"webhook_listen"` // local listen addr, e.g. ":8443"
}

type DiscordConfig struct {
	Token      string `koanf:"token"`
	ShardID    int    `koanf:"shard_id"`    // 0-indexed shard identifier
	ShardCount int    `koanf:"shard_count"` // total shards (0 or 1 = no sharding)
}

func Load() (*Config, error) {
	k := koanf.New(".")

	if err := k.Load(file.Provider("config.yaml"), yaml.Parser()); err != nil {

		if !isFileNotFound(err) {
			return nil, fmt.Errorf("loading config.yaml: %w", err)
		}
	}

	if err := k.Load(env.Provider("BOT_", ".", func(s string) string {
		return strings.ReplaceAll(
			strings.ToLower(strings.TrimPrefix(s, "BOT_")),
			"_", ".",
		)
	}), nil); err != nil {
		return nil, fmt.Errorf("loading env vars: %w", err)
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	if cfg.DefaultLocale == "" {
		cfg.DefaultLocale = "en"
	}
	if cfg.Database.Port == 0 {
		cfg.Database.Port = 5432
	}
	if cfg.Database.SSLMode == "" {
		cfg.Database.SSLMode = "prefer"
	}
	if cfg.Redis.Addr == "" {
		cfg.Redis.Addr = "localhost:6379"
	}
	if cfg.Admin.Port == 0 {
		cfg.Admin.Port = 8080
	}
	if cfg.Admin.ModulesDir == "" {
		cfg.Admin.ModulesDir = "./wasm_modules"
	}
	if cfg.Admin.BlobStore == "" {
		cfg.Admin.BlobStore = "localfs"
	}
	if cfg.Telegram.Mode == "" {
		cfg.Telegram.Mode = "polling"
	}
	if cfg.Discord.ShardCount <= 0 {
		cfg.Discord.ShardCount = 1
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	switch c.Telegram.Mode {
	case "polling", "webhook":
	default:
		return fmt.Errorf("telegram.mode must be \"polling\" or \"webhook\", got %q", c.Telegram.Mode)
	}
	if c.Telegram.Mode == "webhook" && c.Telegram.WebhookURL == "" {
		return fmt.Errorf("telegram.webhook_url is required when telegram.mode=webhook")
	}
	if c.Discord.ShardID < 0 || c.Discord.ShardID >= c.Discord.ShardCount {
		return fmt.Errorf("discord.shard_id (%d) must be in range [0, %d)", c.Discord.ShardID, c.Discord.ShardCount)
	}
	return nil
}

func isFileNotFound(err error) bool {
	return strings.Contains(err.Error(), "no such file") ||
		strings.Contains(err.Error(), "cannot find the file") ||
		strings.Contains(err.Error(), "not found")
}
