package i18n

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var bundle *i18n.Bundle

func Init(defaultLocale string) error {
	tag, err := language.Parse(defaultLocale)
	if err != nil {
		return fmt.Errorf("i18n: invalid default locale %q: %w", defaultLocale, err)
	}

	bundle = i18n.NewBundle(tag)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	entries, err := os.ReadDir("i18n")
	if err != nil {
		if os.IsNotExist(err) {

			return nil
		}
		return fmt.Errorf("i18n: reading i18n directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".toml" {
			continue
		}
		path := filepath.Join("i18n", entry.Name())
		if _, err := bundle.LoadMessageFile(path); err != nil {
			return fmt.Errorf("i18n: loading %s: %w", path, err)
		}
	}

	return nil
}

func Get(key string, locale string, args ...any) string {
	if bundle == nil {
		return key
	}

	localizer := i18n.NewLocalizer(bundle, locale)

	templateData := make(map[string]any, len(args))
	for i, arg := range args {
		templateData[fmt.Sprintf("V%d", i)] = arg
	}

	cfg := &i18n.LocalizeConfig{
		MessageID: key,
	}
	if len(templateData) > 0 {
		cfg.TemplateData = templateData
	}

	msg, err := localizer.Localize(cfg)
	if err != nil {

		return key
	}
	return msg
}
