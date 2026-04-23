package userhttp

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"SuperBotGo/internal/model"
)

type Handler struct {
	sessions       *SessionManager
	tokens         TokenStore
	authenticators []func(r *http.Request) (model.GlobalUserID, bool)
}

func NewHandler(sessions *SessionManager, tokens TokenStore) *Handler {
	h := &Handler{
		sessions: sessions,
		tokens:   tokens,
	}
	if sessions != nil {
		h.authenticators = append(h.authenticators, sessions.Authenticate)
	}
	return h
}

func (h *Handler) AddAuthenticator(fn func(r *http.Request) (model.GlobalUserID, bool)) {
	if fn == nil {
		return
	}
	h.authenticators = append(h.authenticators, fn)
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/session", h.handleSession)
	mux.HandleFunc("POST /api/auth/logout", h.handleLogout)
	mux.HandleFunc("GET /api/auth/tokens", h.handleListTokens)
	mux.HandleFunc("POST /api/auth/tokens", h.handleCreateToken)
	mux.HandleFunc("DELETE /api/auth/tokens/{id}", h.handleDeleteToken)
}

func (h *Handler) handleSession(w http.ResponseWriter, r *http.Request) {
	if h.sessions == nil {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	if userID, ok := h.sessions.Authenticate(r); ok {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "user_id": userID})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
}

func (h *Handler) handleLogout(w http.ResponseWriter, _ *http.Request) {
	if h.sessions != nil {
		h.sessions.ClearSession(w)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) handleListTokens(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireSessionUser(w, r)
	if !ok {
		return
	}
	if h.tokens == nil {
		writeError(w, http.StatusServiceUnavailable, "requires PostgreSQL")
		return
	}

	tokens, err := h.tokens.ListUserTokens(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list user tokens")
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (h *Handler) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireSessionUser(w, r)
	if !ok {
		return
	}
	if h.tokens == nil {
		writeError(w, http.StatusServiceUnavailable, "requires PostgreSQL")
		return
	}

	var body struct {
		Name      string `json:"name"`
		ExpiresAt string `json:"expires_at"`
	}
	if !decodeJSONBody(w, r, &body) {
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	var expiresAt *time.Time
	if body.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, body.ExpiresAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "expires_at must be RFC3339")
			return
		}
		expiresAt = &t
	}

	token, err := h.tokens.CreateUserToken(r.Context(), userID, body.Name, expiresAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user token")
		return
	}
	writeJSON(w, http.StatusCreated, token)
}

func (h *Handler) handleDeleteToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireSessionUser(w, r)
	if !ok {
		return
	}
	if h.tokens == nil {
		writeError(w, http.StatusServiceUnavailable, "requires PostgreSQL")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid token ID")
		return
	}

	if err := h.tokens.DeleteUserToken(r.Context(), userID, id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) requireSessionUser(w http.ResponseWriter, r *http.Request) (model.GlobalUserID, bool) {
	for _, authenticate := range h.authenticators {
		if authenticate == nil {
			continue
		}
		if userID, ok := authenticate(r); ok {
			return userID, true
		}
	}
	writeError(w, http.StatusUnauthorized, "authentication required")
	return 0, false
}
