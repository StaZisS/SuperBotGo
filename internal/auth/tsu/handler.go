package tsu

import (
	"log/slog"
	"net/http"
	"strings"
)

const stateCookieName = "tsu_auth_state"

type Handler struct {
	client       *Client
	stateStore   *StateStore
	linker       *Linker
	secureCookie bool
	logger       *slog.Logger
}

func NewHandler(
	client *Client,
	stateStore *StateStore,
	linker *Linker,
	callbackURL string,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		client:       client,
		stateStore:   stateStore,
		linker:       linker,
		secureCookie: strings.HasPrefix(callbackURL, "https://"),
		logger:       logger,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /oauth/authorize", h.handleLogin)
	mux.HandleFunc("GET /oauth/login", h.handleCallback)
}

// handleLogin validates the state and redirects the user to TSU login page.
func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	if state == "" {
		http.Error(w, "missing state parameter", http.StatusBadRequest)
		return
	}

	if !h.stateStore.Verify(state) {
		http.Error(w, "invalid or expired state", http.StatusBadRequest)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/oauth/",
		MaxAge:   int(stateTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, h.client.LoginURL(), http.StatusFound)
}

// handleCallback is called by TSU after user authentication.
// Exchanges the temporary token for AccountId, links the user.
func (h *Handler) handleCallback(w http.ResponseWriter, r *http.Request) {
	tempToken := r.URL.Query().Get("token")
	if tempToken == "" {
		http.Error(w, "missing token parameter", http.StatusBadRequest)
		return
	}

	cookie, err := r.Cookie(stateCookieName)
	if err != nil {
		h.logger.Warn("tsu callback: missing state cookie")
		http.Error(w, "session expired, please try again", http.StatusBadRequest)
		return
	}

	// Clear the cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    "",
		Path:     "/oauth/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	userID, ok := h.stateStore.Consume(cookie.Value)
	if !ok {
		h.logger.Warn("tsu callback: invalid or expired state")
		http.Error(w, "session expired, please try again", http.StatusBadRequest)
		return
	}

	result, err := h.client.ExchangeToken(r.Context(), tempToken)
	if err != nil {
		h.logger.Error("tsu callback: token exchange failed", slog.Any("error", err))
		http.Error(w, "authentication failed, please try again", http.StatusInternalServerError)
		return
	}

	h.logger.Info("tsu callback: token exchanged",
		slog.Int64("user_id", int64(userID)),
		slog.String("account_id", result.AccountID))

	if err := h.linker.Link(r.Context(), userID, result.AccountID); err != nil {
		h.logger.Error("tsu callback: account linking failed",
			slog.Int64("user_id", int64(userID)),
			slog.Any("error", err))
		http.Error(w, "account linking failed, please try again", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(successHTML))
}

const successHTML = `<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Аккаунт привязан</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            margin: 0;
            background: #f5f5f5;
        }
        .card {
            background: white;
            border-radius: 12px;
            padding: 2rem;
            text-align: center;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
            max-width: 400px;
        }
        .check { font-size: 3rem; margin-bottom: 1rem; }
        h1 { font-size: 1.25rem; margin: 0 0 0.5rem; }
        p { color: #666; margin: 0; }
    </style>
</head>
<body>
    <div class="card">
        <div class="check">&#10004;</div>
        <h1>Аккаунт успешно привязан</h1>
        <p>Можете вернуться в мессенджер.</p>
    </div>
</body>
</html>`
