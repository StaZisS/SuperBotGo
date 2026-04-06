package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// AdminAuthMiddleware protects /api/admin/* endpoints.
// It accepts either a valid session cookie (from email+password login)
// or a Bearer API key (for programmatic access).
// If no admin credentials exist in the database, all requests are allowed (initial setup).
type AdminAuthMiddleware struct {
	apiKey    string
	signer    *sessionSigner
	credStore *PgAdminCredStore
}

func NewAdminAuthMiddleware(auth *AuthHandler) *AdminAuthMiddleware {
	return &AdminAuthMiddleware{
		apiKey:    auth.APIKey(),
		signer:    auth.Signer(),
		credStore: auth.CredStore(),
	}
}

// Wrap returns a handler that enforces authentication on API routes.
func (m *AdminAuthMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Always allow auth endpoints, static files, metrics, and trigger webhooks.
		if strings.HasPrefix(path, "/api/admin/auth/") ||
			!strings.HasPrefix(path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		// If no admin credentials exist, allow all requests (initial setup mode).
		if hasAdmins, err := m.credStore.HasAny(r.Context()); err == nil && !hasAdmins {
			next.ServeHTTP(w, r)
			return
		}

		// Check session cookie first.
		if cookie, err := r.Cookie(sessionCookieName); err == nil {
			if _, ok := m.signer.validate(cookie.Value); ok {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Fall back to Bearer API key (for programmatic access).
		if m.apiKey != "" {
			if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				token := auth[len("Bearer "):]
				if subtle.ConstantTimeCompare([]byte(token), []byte(m.apiKey)) == 1 {
					next.ServeHTTP(w, r)
					return
				}
			}
		}

		writeError(w, http.StatusUnauthorized, "authentication required")
	})
}
