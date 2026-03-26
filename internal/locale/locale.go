package locale

import "sync/atomic"

var defaultLocale atomic.Value

func init() {
	defaultLocale.Store("en")
}

// SetDefault sets the global default locale (called once at startup from config).
func SetDefault(loc string) {
	if loc != "" {
		defaultLocale.Store(loc)
	}
}

// Default returns the current default locale.
func Default() string {
	return defaultLocale.Load().(string)
}
