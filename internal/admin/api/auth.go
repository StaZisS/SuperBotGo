package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	sessionCookieName = "admin_session"
	sessionTTL        = 24 * time.Hour
)

// sessionSigner creates and validates HMAC-signed session tokens
// containing a user ID and expiration.
type sessionSigner struct {
	key []byte
}

func newSessionSigner(secret string) *sessionSigner {
	if secret == "" {
		// Generate a random key when no secret is configured.
		// Sessions won't survive restarts, which is acceptable for dev.
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			panic("failed to generate session key: " + err.Error())
		}
		slog.Warn("admin auth: no session secret configured — using random key (sessions will not survive restarts)")
		return &sessionSigner{key: b}
	}
	return &sessionSigner{key: []byte(secret)}
}

// createToken returns a token in the format "userID:expiry:signature".
func (s *sessionSigner) createToken(userID int64, ttl time.Duration) string {
	expiry := strconv.FormatInt(time.Now().Add(ttl).Unix(), 10)
	uid := strconv.FormatInt(userID, 10)
	payload := uid + ":" + expiry
	return payload + ":" + s.sign(payload)
}

func (s *sessionSigner) sign(data string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

// validate checks that the token is well-formed, not expired, and correctly signed.
// Returns the user ID on success.
func (s *sessionSigner) validate(token string) (int64, bool) {
	parts := strings.SplitN(token, ":", 3)
	if len(parts) != 3 {
		return 0, false
	}
	uidStr, expiryStr, sig := parts[0], parts[1], parts[2]

	expiry, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil || time.Now().Unix() > expiry {
		return 0, false
	}

	userID, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil {
		return 0, false
	}

	expected := s.sign(uidStr + ":" + expiryStr)
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return 0, false
	}
	return userID, true
}

// AuthHandler handles login / logout / session-check endpoints.
type AuthHandler struct {
	apiKey    string
	signer    *sessionSigner
	credStore *PgAdminCredStore
}

func NewAuthHandler(apiKey string, credStore *PgAdminCredStore) *AuthHandler {
	return &AuthHandler{
		apiKey:    apiKey,
		signer:    newSessionSigner(apiKey),
		credStore: credStore,
	}
}

func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/admin/auth/login", h.handleLogin)
	mux.HandleFunc("POST /api/admin/auth/logout", h.handleLogout)
	mux.HandleFunc("GET /api/admin/auth/check", h.handleCheck)
	mux.HandleFunc("PUT /api/admin/auth/password", h.handleChangePassword)
}

func (h *AuthHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Email == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	userID, err := h.credStore.Authenticate(r.Context(), body.Email, body.Password)
	if err != nil {
		slog.Warn("admin auth: failed login attempt", "email", body.Email, "remote", r.RemoteAddr)
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	token := h.signer.createToken(userID, sessionTTL)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	slog.Info("admin auth: successful login", "user_id", userID, "email", body.Email, "remote", r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user_id": userID})
}

func (h *AuthHandler) handleLogout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *AuthHandler) handleCheck(w http.ResponseWriter, r *http.Request) {
	// If no admin credentials exist at all, auth is disabled (initial setup).
	hasAdmins, err := h.credStore.HasAny(r.Context())
	if err != nil {
		slog.Error("admin auth: failed to check admin credentials", "error", err)
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	if !hasAdmins {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "setup_required": true})
		return
	}

	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		if userID, ok := h.signer.validate(cookie.Value); ok {
			writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "user_id": userID})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
}

func (h *AuthHandler) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	userID, ok := h.signer.validate(cookie.Value)
	if !ok {
		writeError(w, http.StatusUnauthorized, "session expired")
		return
	}

	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(body.NewPassword) < 8 {
		writeError(w, http.StatusBadRequest, "new password must be at least 8 characters")
		return
	}

	// Verify current password via the credential store.
	if err := h.credStore.VerifyPassword(r.Context(), userID, body.CurrentPassword); err != nil {
		writeError(w, http.StatusForbidden, "current password is incorrect")
		return
	}

	if err := h.credStore.UpdatePassword(r.Context(), userID, body.NewPassword); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	slog.Info("admin auth: password changed", "user_id", userID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// Signer returns the session signer (used by the middleware).
func (h *AuthHandler) Signer() *sessionSigner {
	return h.signer
}

// APIKey returns the configured API key (used by the middleware).
func (h *AuthHandler) APIKey() string {
	return h.apiKey
}

// CredStore returns the credentials store (used by the middleware for HasAny check).
func (h *AuthHandler) CredStore() *PgAdminCredStore {
	return h.credStore
}
