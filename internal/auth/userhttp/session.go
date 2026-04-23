package userhttp

import (
	"net/http"
	"time"

	"SuperBotGo/internal/auth/session"
	"SuperBotGo/internal/model"
)

const (
	SessionCookieName = "user_session"
	SessionTTL        = 24 * time.Hour
)

type SessionManager struct {
	signer       *session.Signer
	secureCookie bool
}

func NewSessionManager(secret string, secureCookie bool) *SessionManager {
	return &SessionManager{
		signer:       session.NewSigner(secret, "user auth"),
		secureCookie: secureCookie,
	}
}

func (m *SessionManager) Authenticate(r *http.Request) (model.GlobalUserID, bool) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return 0, false
	}
	userID, ok := m.signer.Validate(cookie.Value)
	if !ok {
		return 0, false
	}
	return model.GlobalUserID(userID), true
}

func (m *SessionManager) SetSession(w http.ResponseWriter, userID model.GlobalUserID) {
	token := m.signer.CreateToken(int64(userID), SessionTTL)
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   m.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}

func (m *SessionManager) ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}
