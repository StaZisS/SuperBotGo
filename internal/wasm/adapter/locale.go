package adapter

import "SuperBotGo/internal/locale"

// ResolveLocalizedText picks the best text from a locale→text map given the
// target locale. Fallback order: exact match → language prefix → default locale → first value.
func ResolveLocalizedText(texts map[string]string, loc string) string {
	return locale.ResolveText(texts, loc)
}
