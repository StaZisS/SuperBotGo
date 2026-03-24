package adapter

import "strings"

// ResolveLocalizedText picks the best text from a locale→text map given the
// target locale. Fallback order: exact match → language prefix → "en" → first value.
func ResolveLocalizedText(texts map[string]string, locale string) string {
	if len(texts) == 0 {
		return ""
	}

	// Exact match.
	if text, ok := texts[locale]; ok {
		return text
	}

	// Language prefix match (e.g. "ru-RU" → "ru").
	if idx := strings.IndexByte(locale, '-'); idx > 0 {
		lang := locale[:idx]
		if text, ok := texts[lang]; ok {
			return text
		}
	}

	// Fallback to English.
	if text, ok := texts["en"]; ok {
		return text
	}

	// Last resort: first value in map.
	for _, text := range texts {
		return text
	}
	return ""
}
