package userhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"SuperBotGo/internal/model"
)

func TestHandlerSession(t *testing.T) {
	sessions := NewSessionManager("test-secret", false)
	handler := NewHandler(sessions, nil)

	req := authenticatedRequest(t, sessions, http.MethodGet, "/api/auth/session", nil)
	rec := httptest.NewRecorder()

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["authenticated"] != true {
		t.Fatalf("authenticated = %#v, want true", body["authenticated"])
	}
}

func TestHandlerCreateListDeleteTokens(t *testing.T) {
	sessions := NewSessionManager("test-secret", false)
	store := &fakeTokenStore{}
	handler := NewHandler(sessions, store)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	createReq := authenticatedRequest(t, sessions, http.MethodPost, "/api/auth/tokens", bytes.NewBufferString(`{"name":"CLI token"}`))
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", createRec.Code, http.StatusCreated)
	}
	if got := store.lastCreatedUserID; got != 42 {
		t.Fatalf("created token for user %d, want 42", got)
	}

	listReq := authenticatedRequest(t, sessions, http.MethodGet, "/api/auth/tokens", nil)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listRec.Code, http.StatusOK)
	}
	var records []UserTokenRecord
	if err := json.Unmarshal(listRec.Body.Bytes(), &records); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(records) != 1 || records[0].Name != "CLI token" {
		t.Fatalf("unexpected list response: %#v", records)
	}

	deleteReq := authenticatedRequest(t, sessions, http.MethodDelete, "/api/auth/tokens/1", nil)
	deleteRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteRec, deleteReq)

	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want %d", deleteRec.Code, http.StatusOK)
	}
	if got := store.lastDeletedUserID; got != 42 {
		t.Fatalf("deleted token for user %d, want 42", got)
	}
}

func TestHandlerCreateTokenRequiresSession(t *testing.T) {
	handler := NewHandler(NewSessionManager("test-secret", false), &fakeTokenStore{})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/tokens", bytes.NewBufferString(`{"name":"CLI token"}`))
	rec := httptest.NewRecorder()

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandlerCreateTokenAllowsFallbackAuthenticator(t *testing.T) {
	handler := NewHandler(nil, &fakeTokenStore{})
	handler.AddAuthenticator(func(_ *http.Request) (model.GlobalUserID, bool) {
		return 77, true
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/tokens", bytes.NewBufferString(`{"name":"CLI token"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
}

type fakeTokenStore struct {
	tokens            []UserTokenRecord
	lastCreatedUserID model.GlobalUserID
	lastDeletedUserID model.GlobalUserID
}

func (s *fakeTokenStore) ListUserTokens(_ context.Context, _ model.GlobalUserID) ([]UserTokenRecord, error) {
	return append([]UserTokenRecord(nil), s.tokens...), nil
}

func (s *fakeTokenStore) CreateUserToken(_ context.Context, userID model.GlobalUserID, name string, expiresAt *time.Time) (CreatedUserToken, error) {
	s.lastCreatedUserID = userID
	rec := UserTokenRecord{
		ID:        int64(len(s.tokens) + 1),
		PublicID:  fmt.Sprintf("pub-%d", len(s.tokens)+1),
		Name:      name,
		Active:    true,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.tokens = append(s.tokens, rec)
	return CreatedUserToken{
		UserTokenRecord: rec,
		Token:           "sbuk_demo.secret",
	}, nil
}

func (s *fakeTokenStore) DeleteUserToken(_ context.Context, userID model.GlobalUserID, id int64) error {
	s.lastDeletedUserID = userID
	for i, token := range s.tokens {
		if token.ID == id {
			s.tokens = append(s.tokens[:i], s.tokens[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("token %d not found", id)
}

func (s *fakeTokenStore) AuthenticateUserToken(_ context.Context, _ string) (model.GlobalUserID, bool, error) {
	return 0, false, nil
}

func authenticatedRequest(t *testing.T, sessions *SessionManager, method, target string, body *bytes.Buffer) *http.Request {
	t.Helper()

	var reader *bytes.Buffer
	if body != nil {
		reader = body
	} else {
		reader = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, target, reader)
	rec := httptest.NewRecorder()
	sessions.SetSession(rec, 42)
	for _, cookie := range rec.Result().Cookies() {
		req.AddCookie(cookie)
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}
