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

	// Exact match.
	if text, ok := texts[loc]; ok {
		return text
	}

	// Language prefix match (e.g. "ru-RU" → "ru").
	if idx := strings.IndexByte(loc, '-'); idx > 0 {
		lang := loc[:idx]
		if text, ok := texts[lang]; ok {
			return text
		}
	}

	// Fallback to default locale.
	if text, ok := texts[locale.Default()]; ok {
		return text
	}

	// Last resort: first value in map.
	for _, text := range texts {
		return text
	}
	return ""
}
