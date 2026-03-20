package runtime

// Config holds settings for the Wasm runtime.
type Config struct {
	// CacheDir is the directory for AOT compilation cache.
	CacheDir string
	// DefaultMemoryLimitPages is the maximum number of Wasm memory pages (64KB each).
	// Default: 256 (16MB).
	DefaultMemoryLimitPages uint32
	// DefaultTimeoutSeconds is the execution timeout per call. Default: 5.
	DefaultTimeoutSeconds int
}

// withDefaults returns a copy of the config with zero values replaced by defaults.
func (c Config) withDefaults() Config {
	if c.DefaultMemoryLimitPages == 0 {
		c.DefaultMemoryLimitPages = 256
	}
	if c.DefaultTimeoutSeconds == 0 {
		c.DefaultTimeoutSeconds = 5
	}
	return c
}
