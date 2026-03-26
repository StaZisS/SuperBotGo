package wasmplugin

import (
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// LoadFS loads translations from TOML files in an embedded filesystem.
// Files must be named {locale}.toml (e.g. "en.toml", "ru.toml") and use
// flat key-value format identical to the host's i18n files:
//
//	# Comment
//	"schedule" = "Schedule"
//	"building" = "Building"
//	greeting = "Hello, {{.Name}}!"
//
// Usage with embed.FS:
//
//	//go:embed i18n/*.toml
//	var i18nFS embed.FS
//
//	var cat = wasmplugin.NewCatalog("en").
//	    Merge(wasmplugin.CommonMessages).
//	    LoadFS(i18nFS, "i18n")
func (c *Catalog) LoadFS(fsys fs.FS, dir string) *Catalog {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return c
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".toml") {
			continue
		}
		locale := strings.TrimSuffix(name, ".toml")
		data, err := fs.ReadFile(fsys, path.Join(dir, name))
		if err != nil {
			continue
		}
		kv, err := parseFlatTOML(data)
		if err != nil {
			continue
		}
		c.Add(locale, kv)
	}
	return c
}

// parseFlatTOML parses a flat TOML file (key = "value" pairs only).
// Supports:
//   - Quoted keys:   "schedule.header" = "Schedule"
//   - Bare keys:     greeting = "Hello"
//   - Comments:      # this is a comment
//   - Empty lines
//   - Basic escape sequences in values: \n \t \\ \"
//
// Does NOT support: tables, arrays, multi-line strings, inline tables,
// or any other TOML features beyond flat key-value pairs.
func parseFlatTOML(data []byte) (map[string]string, error) {
	result := make(map[string]string)
	lines := strings.Split(string(data), "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}

		key, value, err := parseTOMLLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		result[key] = value
	}
	return result, nil
}

// parseTOMLLine parses a single "key = value" line.
func parseTOMLLine(line string) (string, string, error) {
	var key string
	rest := line

	if rest[0] == '"' {
		// Quoted key: "some.key" = "value"
		end := findClosingQuote(rest, 1)
		if end < 0 {
			return "", "", fmt.Errorf("unterminated quoted key")
		}
		key = rest[1:end]
		rest = strings.TrimSpace(rest[end+1:])
	} else {
		// Bare key: key = "value"
		eqIdx := strings.IndexByte(rest, '=')
		if eqIdx < 0 {
			return "", "", fmt.Errorf("missing '='")
		}
		key = strings.TrimSpace(rest[:eqIdx])
		rest = rest[eqIdx:]
	}

	if len(rest) == 0 || rest[0] != '=' {
		return "", "", fmt.Errorf("missing '='")
	}
	rest = strings.TrimSpace(rest[1:]) // skip '='

	if len(rest) == 0 || rest[0] != '"' {
		return "", "", fmt.Errorf("value must be a quoted string")
	}

	end := findClosingQuote(rest, 1)
	if end < 0 {
		return "", "", fmt.Errorf("unterminated string value")
	}
	value := unescapeTOML(rest[1:end])

	return key, value, nil
}

// findClosingQuote finds the index of the closing '"' starting search at pos,
// handling \" escapes.
func findClosingQuote(s string, pos int) int {
	for i := pos; i < len(s); i++ {
		if s[i] == '\\' {
			i++ // skip escaped char
			continue
		}
		if s[i] == '"' {
			return i
		}
	}
	return -1
}

// unescapeTOML handles basic TOML string escape sequences.
func unescapeTOML(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case '\\':
				b.WriteByte('\\')
			case '"':
				b.WriteByte('"')
			case 'r':
				b.WriteByte('\r')
			default:
				b.WriteByte('\\')
				b.WriteByte(s[i])
			}
		} else {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
