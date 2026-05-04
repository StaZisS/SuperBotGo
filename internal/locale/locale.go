package locale

import (
	"strings"
	"sync/atomic"
)

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

// ResolveText picks the best text from a locale->text map.
// Fallback order: exact match -> language prefix -> default locale -> first value.
func ResolveText(texts map[string]string, loc string) string {
	if len(texts) == 0 {
		return ""
	}

	if text, ok := texts[loc]; ok {
		return text
	}

	if idx := strings.IndexByte(loc, '-'); idx > 0 {
		lang := loc[:idx]
		if text, ok := texts[lang]; ok {
			return text
		}
	}

	if text, ok := texts[Default()]; ok {
		return text
	}

	for _, text := range texts {
		return text
	}
	return ""
}
