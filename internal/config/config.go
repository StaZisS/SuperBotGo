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
	DefaultLocale  string               `koanf:"default_locale"`
	Database       DatabaseConfig       `koanf:"database"`
	Redis          RedisConfig          `koanf:"redis"`
	Telegram       TelegramConfig       `koanf:"telegram"`
	Discord        DiscordConfig        `koanf:"discord"`
	VK             VKConfig             `koanf:"vk"`
	Mattermost     MattermostConfig     `koanf:"mattermost"`
	Admin          AdminConfig          `koanf:"admin"`
	UserAuth       UserAuthConfig       `koanf:"user_auth"`
	WASM           WASMConfig           `koanf:"wasm"`
	SpiceDB        SpiceDBConfig        `koanf:"spicedb"`
	UniversitySync UniversitySyncConfig `koanf:"university_sync"`
	FileStore      FileStoreConfig      `koanf:"filestore"`
	TsuAccounts    TsuAccountsConfig    `koanf:"tsu_accounts"`
	SMTP           SMTPConfig           `koanf:"smtp"`
}

type WASMConfig struct {
	ReconfigureEnabled *bool  `koanf:"reconfigure_enabled"`
	RPCEnabled         *bool  `koanf:"rpc_enabled"`
	EventsBackend      string `koanf:"events_backend"`
	StrictMigrate      *bool  `koanf:"strict_migrate"`
	HTTPPolicyEnabled  *bool  `koanf:"http_policy_enabled"`
}

func (c WASMConfig) ReconfigureEnabledValue() bool {
	return c.ReconfigureEnabled == nil || *c.ReconfigureEnabled
}

func (c WASMConfig) RPCEnabledValue() bool {
	return c.RPCEnabled != nil && *c.RPCEnabled
}

func (c WASMConfig) StrictMigrateValue() bool {
	return c.StrictMigrate == nil || *c.StrictMigrate
}

func (c WASMConfig) HTTPPolicyEnabledValue() bool {
	return c.HTTPPolicyEnabled != nil && *c.HTTPPolicyEnabled
}

type TsuAccountsConfig struct {
	ApplicationID string `koanf:"application_id"` // BOT_TSU__ACCOUNTS_APPLICATION__ID
	SecretKey     string `koanf:"secret_key"`     // BOT_TSU__ACCOUNTS_SECRET__KEY
	CallbackURL   string `koanf:"callback_url"`   // public URL for TSU redirect
	BaseURL       string `koanf:"base_url"`       // https://accounts.tsu.ru
}

type SMTPConfig struct {
	Host     string `koanf:"host"`
	Port     int    `koanf:"port"`
	Username string `koanf:"username"`
	Password string `koanf:"password"`
	From     string `koanf:"from"`
}

type UniversitySyncConfig struct {
	Enabled  bool   `koanf:"enabled"`
	Interval string `koanf:"interval"` // e.g. "1h", "30m", "24h"
	BaseURL  string `koanf:"base_url"` // external system API base URL
	Token    string `koanf:"token"`    // auth token for external system
}

type SpiceDBConfig struct {
	Endpoint string `koanf:"endpoint"` // Например: localhost:50051
	Token    string `koanf:"token"`    // Preshared key
	Insecure bool   `koanf:"insecure"` // true для локальной разработки без TLS
}

type AdminConfig struct {
	Port       int      `koanf:"port"`
	ModulesDir string   `koanf:"modules_dir"`
	BlobStore  string   `koanf:"blob_store"`
	APIKey     string   `koanf:"api_key"`
	S3         S3Config `koanf:"s3"`
}

type UserAuthConfig struct {
	SessionSecret string `koanf:"session_secret"`
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
	Mode          string `koanf:"mode"`
	WebhookURL    string `koanf:"webhook_url"`
	WebhookSecret string `koanf:"webhook_secret"`
	WebhookListen string `koanf:"webhook_listen"`
}

type DiscordConfig struct {
	Token      string `koanf:"token"`
	ShardID    int    `koanf:"shard_id"`
	ShardCount int    `koanf:"shard_count"`
}

type VKConfig struct {
	Token        string `koanf:"token"`
	Mode         string `koanf:"mode"`
	CallbackURL  string `koanf:"callback_url"`
	CallbackPath string `koanf:"callback_path"`
}

type MattermostConfig struct {
	URL           string `koanf:"url"`
	Token         string `koanf:"token"`
	ActionsURL    string `koanf:"actions_url"`
	ActionsPath   string `koanf:"actions_path"`
	ActionsSecret string `koanf:"actions_secret"`
}

type FileStoreConfig struct {
	S3          S3Config `koanf:"s3"`
	DefaultTTL  string   `koanf:"default_ttl"`   // e.g. "24h", "0" for no expiry
	MaxFileSize int64    `koanf:"max_file_size"` // max file size in bytes (default 50MB)
}

func Load() (*Config, error) {
	k := koanf.New(".")

	if err := k.Load(file.Provider("config.yaml"), yaml.Parser()); err != nil {
		if !isFileNotFound(err) {
			return nil, fmt.Errorf("loading config.yaml: %w", err)
		}
	}

	if err := k.Load(env.Provider("BOT_", ".", func(s string) string {
		key := strings.ToLower(strings.TrimPrefix(s, "BOT_"))
		// Double underscore (__) → literal underscore in field name
		// Single underscore (_) → dot (koanf level separator)
		// Example: BOT_ADMIN_S3_ACCESS__KEY → admin.s3.access_key
		key = strings.ReplaceAll(key, "__", "\x00")
		key = strings.ReplaceAll(key, "_", ".")
		key = strings.ReplaceAll(key, "\x00", "_")
		return key
	}), nil); err != nil {
		return nil, fmt.Errorf("loading env vars: %w", err)
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	// Defaults
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
		cfg.Admin.BlobStore = "s3"
	}
	if cfg.WASM.EventsBackend == "" {
		cfg.WASM.EventsBackend = "memory"
	}
	if cfg.Telegram.Mode == "" {
		cfg.Telegram.Mode = "polling"
	}
	if cfg.VK.Mode == "" {
		cfg.VK.Mode = "longpoll"
	}
	if cfg.VK.CallbackPath == "" {
		cfg.VK.CallbackPath = "/vk/callback"
	}
	if cfg.Discord.ShardCount <= 0 {
		cfg.Discord.ShardCount = 1
	}
	if cfg.Mattermost.ActionsPath == "" {
		cfg.Mattermost.ActionsPath = "/mattermost/actions"
	}
	if cfg.FileStore.DefaultTTL == "" {
		cfg.FileStore.DefaultTTL = "24h"
	}
	if cfg.FileStore.MaxFileSize <= 0 {
		cfg.FileStore.MaxFileSize = 50 * 1024 * 1024 // 50MB
	}

	// Defaults for SpiceDB
	if cfg.SpiceDB.Endpoint == "" {
		cfg.SpiceDB.Endpoint = "localhost:50051"
	}
	if cfg.SpiceDB.Token == "" {
		cfg.SpiceDB.Token = "my-secret-token" // Дефолтный токен для Docker
	}

	if cfg.TsuAccounts.BaseURL == "" {
		cfg.TsuAccounts.BaseURL = "https://accounts.tsu.ru"
	}
	if cfg.SMTP.Port == 0 {
		cfg.SMTP.Port = 587
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
	switch c.VK.Mode {
	case "longpoll", "callback":
	default:
		return fmt.Errorf("vk.mode must be \"longpoll\" or \"callback\", got %q", c.VK.Mode)
	}
	if c.VK.Mode == "callback" && c.VK.CallbackURL == "" {
		return fmt.Errorf("vk.callback_url is required when vk.mode=callback")
	}
	if c.VK.CallbackPath != "" && !strings.HasPrefix(c.VK.CallbackPath, "/") {
		return fmt.Errorf("vk.callback_path must start with \"/\", got %q", c.VK.CallbackPath)
	}
	if c.Discord.ShardID < 0 || c.Discord.ShardID >= c.Discord.ShardCount {
		return fmt.Errorf("discord.shard_id (%d) must be in range [0, %d)", c.Discord.ShardID, c.Discord.ShardCount)
	}
	if (c.Mattermost.URL == "") != (c.Mattermost.Token == "") {
		return fmt.Errorf("mattermost.url and mattermost.token must be set together")
	}
	if (c.Mattermost.ActionsURL == "") != (c.Mattermost.ActionsSecret == "") {
		return fmt.Errorf("mattermost.actions_url and mattermost.actions_secret must be set together")
	}
	if (c.SMTP.Host == "") != (c.SMTP.From == "") {
		return fmt.Errorf("smtp.host and smtp.from must be set together")
	}
	if c.Mattermost.ActionsPath != "" && !strings.HasPrefix(c.Mattermost.ActionsPath, "/") {
		return fmt.Errorf("mattermost.actions_path must start with \"/\", got %q", c.Mattermost.ActionsPath)
	}
	switch c.Admin.BlobStore {
	case "", "s3":
	default:
		return fmt.Errorf("admin.blob_store must be \"s3\", got %q", c.Admin.BlobStore)
	}
	switch c.WASM.EventsBackend {
	case "", "memory", "postgres":
	default:
		return fmt.Errorf("wasm.events_backend must be \"memory\" or \"postgres\", got %q", c.WASM.EventsBackend)
	}
	return nil
}

func isFileNotFound(err error) bool {
	return strings.Contains(err.Error(), "no such file") ||
		strings.Contains(err.Error(), "cannot find the file") ||
		strings.Contains(err.Error(), "not found")
}
