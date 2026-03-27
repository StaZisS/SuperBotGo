package adapter

import (
	"strings"

	"SuperBotGo/internal/locale"
)

// ResolveLocalizedText picks the best text from a locale→text map given the
// target locale. Fallback order: exact match → language prefix → default locale → first value.
func ResolveLocalizedText(texts map[string]string, loc string) string {
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

	if text, ok := texts[locale.Default()]; ok {
		return text
	}

	for _, text := range texts {
		return text
	}
	return ""
}
