package runtime

type Config struct {
	CacheDir                string
	DefaultMemoryLimitPages uint32
	DefaultTimeoutSeconds   int
}

func (c Config) withDefaults() Config {
	if c.DefaultMemoryLimitPages == 0 {
		c.DefaultMemoryLimitPages = 256
	}
	if c.DefaultTimeoutSeconds == 0 {
		c.DefaultTimeoutSeconds = 5
	}
	return c
}
