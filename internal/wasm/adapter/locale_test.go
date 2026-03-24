package adapter

import "testing"

func TestResolveLocalizedText(t *testing.T) {
	tests := []struct {
		name   string
		texts  map[string]string
		locale string
		want   string
	}{
		{
			name:   "exact match",
			texts:  map[string]string{"en": "Hello", "ru": "Привет"},
			locale: "ru",
			want:   "Привет",
		},
		{
			name:   "prefix match",
			texts:  map[string]string{"en": "Hello", "ru": "Привет"},
			locale: "ru-RU",
			want:   "Привет",
		},
		{
			name:   "fallback to en",
			texts:  map[string]string{"en": "Hello", "de": "Hallo"},
			locale: "fr",
			want:   "Hello",
		},
		{
			name:   "fallback to first when no en",
			texts:  map[string]string{"de": "Hallo"},
			locale: "fr",
			want:   "Hallo",
		},
		{
			name:   "empty map",
			texts:  map[string]string{},
			locale: "en",
			want:   "",
		},
		{
			name:   "nil map",
			texts:  nil,
			locale: "en",
			want:   "",
		},
		{
			name:   "empty locale falls back to en",
			texts:  map[string]string{"en": "Hello", "ru": "Привет"},
			locale: "",
			want:   "Hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveLocalizedText(tt.texts, tt.locale)
			if got != tt.want {
				t.Errorf("ResolveLocalizedText(%v, %q) = %q, want %q", tt.texts, tt.locale, got, tt.want)
			}
		})
	}
}
