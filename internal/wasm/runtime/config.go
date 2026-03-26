package runtime

import "context"

const (
	DefaultPluginTimeoutSeconds = 5

	DefaultHostHTTPTimeoutSeconds = 4

	DefaultHostEventTimeoutSeconds = 3

	DefaultHostCallPluginTimeoutSeconds = 4

	DefaultHostNotifyTimeoutSeconds = 4
)

type PluginTimeoutOverrideKey struct{}

func pluginTimeoutFromContext(ctx context.Context, configSeconds int) int {
	if override, ok := ctx.Value(PluginTimeoutOverrideKey{}).(int); ok && override > 0 {
		return override
	}
	if configSeconds > 0 {
		return configSeconds
	}
	return DefaultPluginTimeoutSeconds
}

type Config struct {
	CacheDir                string
	DefaultMemoryLimitPages uint32
	DefaultTimeoutSeconds   int

	PoolMaxConcurrency int
}

func (c Config) withDefaults() Config {
	if c.DefaultMemoryLimitPages == 0 {
		c.DefaultMemoryLimitPages = 256
	}
	if c.DefaultTimeoutSeconds == 0 {
		c.DefaultTimeoutSeconds = DefaultPluginTimeoutSeconds
	}
	return c
}

func (c Config) PoolConfig() *PoolConfig {
	if c.PoolMaxConcurrency == 0 {
		return nil
	}
	return &PoolConfig{MaxConcurrency: c.PoolMaxConcurrency}
}
