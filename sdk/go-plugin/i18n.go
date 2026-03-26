package wasmplugin

import (
	"fmt"
	"strings"
)

// Catalog holds translations for a plugin. It is safe for concurrent reads
// after initialization (WASM is single-threaded, so this is always the case).
//
// The primary method is [Catalog.L] which returns a map of all locale variants
// for a given key. The host then resolves the target locale per-recipient.
// This allows a plugin to form one message and broadcast it to users with
// different locale preferences — the host picks the right variant.
//
// For cases where the plugin must produce a single-locale string (e.g. composing
// complex text), use [Catalog.T] or [Catalog.Tr].
type Catalog struct {
	defaultLocale string
	messages      map[string]map[string]string // locale -> key -> text
}

// NewCatalog creates a translation catalog with the given default/fallback locale.
func NewCatalog(defaultLocale string) *Catalog {
	return &Catalog{
		defaultLocale: defaultLocale,
		messages:      make(map[string]map[string]string),
	}
}

// Add registers translations for a locale. Can be called multiple times for the
// same locale — new keys are merged, existing keys are overwritten.
func (c *Catalog) Add(locale string, translations map[string]string) *Catalog {
	if c.messages[locale] == nil {
		c.messages[locale] = make(map[string]string, len(translations))
	}
	for k, v := range translations {
		c.messages[locale][k] = v
	}
	return c
}

// Merge copies translations from another catalog. Keys that already exist in c
// are NOT overwritten — the receiver's translations take priority. This allows
// a plugin to override common translations (e.g. customize "yes" to "Agree").
func (c *Catalog) Merge(other *Catalog) *Catalog {
	for locale, msgs := range other.messages {
		if c.messages[locale] == nil {
			c.messages[locale] = make(map[string]string, len(msgs))
		}
		for k, v := range msgs {
			if _, exists := c.messages[locale][k]; !exists {
				c.messages[locale][k] = v
			}
		}
	}
	return c
}

// L returns a map[string]string with all locale variants for the given key.
// This is the primary method — use it with [StepBuilder.LocalizedText],
// [EventContext.ReplyLocalized], [Option.Labels], etc. The host resolves the
// target locale per-recipient, so the plugin does not need to know the user's
// locale.
//
// Optional args are interpolated into each locale's template:
//   - Named pairs: L("greeting", "Name", name, "Count", n)
//     replaces {{.Name}} and {{.Count}} in each locale's text.
//   - Positional: L("msg", val0, val1) replaces {{.V0}}, {{.V1}}.
func (c *Catalog) L(key string, args ...any) map[string]string {
	result := make(map[string]string, len(c.messages))
	for locale, msgs := range c.messages {
		text, ok := msgs[key]
		if !ok {
			continue
		}
		if len(args) > 0 {
			text = interpolate(text, args)
		}
		result[locale] = text
	}
	// Ensure at least the default locale is present.
	if len(result) == 0 {
		result[c.defaultLocale] = key
	}
	return result
}

// T returns the translated string for a specific locale.
// Fallback order: exact locale -> language prefix (e.g. "ru-RU" -> "ru") ->
// default locale -> key itself.
//
// Optional args work the same as in [Catalog.L].
func (c *Catalog) T(locale, key string, args ...any) string {
	text := c.resolve(locale, key)
	if len(args) > 0 {
		text = interpolate(text, args)
	}
	return text
}

// Tr returns a translator function bound to a specific locale. Convenient
// in handlers where the locale is known once and used many times:
//
//	tr := cat.Tr(ctx.Locale())
//	tr("building")  // "Корпус"
//	tr("room")      // "Аудитория"
func (c *Catalog) Tr(locale string) func(key string, args ...any) string {
	return func(key string, args ...any) string {
		return c.T(locale, key, args...)
	}
}

// Opt creates an Option with localized labels for the given key. The host
// resolves the target locale from Labels; Label is set to the default locale
// text as a fallback for older hosts.
func (c *Catalog) Opt(key, value string, args ...any) Option {
	labels := c.L(key, args...)
	label := labels[c.defaultLocale]
	if label == "" {
		label = key
	}
	return Option{
		Label:  label,
		Labels: labels,
		Value:  value,
	}
}

// resolve finds the best translation for a locale+key, following the fallback
// chain: exact match -> language prefix -> default locale -> key.
func (c *Catalog) resolve(locale, key string) string {
	// Exact match.
	if msgs, ok := c.messages[locale]; ok {
		if text, ok := msgs[key]; ok {
			return text
		}
	}
	// Language prefix (e.g. "ru-RU" -> "ru").
	if idx := strings.IndexByte(locale, '-'); idx > 0 {
		lang := locale[:idx]
		if msgs, ok := c.messages[lang]; ok {
			if text, ok := msgs[key]; ok {
				return text
			}
		}
	}
	// Default locale.
	if locale != c.defaultLocale {
		if msgs, ok := c.messages[c.defaultLocale]; ok {
			if text, ok := msgs[key]; ok {
				return text
			}
		}
	}
	// Key itself as last resort.
	return key
}

// interpolate replaces template placeholders in text with args.
// Supports two modes:
//   - Named pairs: ("Name", val, "Count", val) -> {{.Name}}, {{.Count}}
//   - Positional: (val0, val1) -> {{.V0}}, {{.V1}}
func interpolate(text string, args []any) string {
	if len(args) == 0 {
		return text
	}
	// Named pairs: first arg is a string, we have at least 2 args.
	if len(args) >= 2 {
		if _, ok := args[0].(string); ok {
			for i := 0; i+1 < len(args); i += 2 {
				name, _ := args[i].(string)
				val := fmt.Sprintf("%v", args[i+1])
				text = strings.ReplaceAll(text, "{{."+name+"}}", val)
			}
			return text
		}
	}
	// Positional: V0, V1, ...
	for i, arg := range args {
		placeholder := fmt.Sprintf("{{.V%d}}", i)
		text = strings.ReplaceAll(text, placeholder, fmt.Sprintf("%v", arg))
	}
	return text
}

// CommonMessages is a pre-built catalog with common UI translations.
// Plugin authors can merge it into their catalog:
//
//	var cat = wasmplugin.NewCatalog("en").
//	    Merge(wasmplugin.CommonMessages).
//	    Add("en", map[string]string{...}).
//	    Add("ru", map[string]string{...})
var CommonMessages = NewCatalog("en").
	Add("en", map[string]string{
		"yes":     "Yes",
		"no":      "No",
		"back":    "Back",
		"next":    "Next",
		"cancel":  "Cancel",
		"done":    "Done",
		"error":   "An error occurred",
		"loading": "Loading...",
		"save":    "Save",
		"delete":  "Delete",
		"edit":    "Edit",
		"search":  "Search",
		"close":   "Close",
		"confirm": "Confirm",
		"retry":   "Retry",
	}).
	Add("ru", map[string]string{
		"yes":     "Да",
		"no":      "Нет",
		"back":    "Назад",
		"next":    "Далее",
		"cancel":  "Отмена",
		"done":    "Готово",
		"error":   "Произошла ошибка",
		"loading": "Загрузка...",
		"save":    "Сохранить",
		"delete":  "Удалить",
		"edit":    "Редактировать",
		"search":  "Поиск",
		"close":   "Закрыть",
		"confirm": "Подтвердить",
		"retry":   "Повторить",
	})
