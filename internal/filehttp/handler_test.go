package filehttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"SuperBotGo/internal/auth/userhttp"
	"SuperBotGo/internal/filestore"
	"SuperBotGo/internal/model"
)

func TestHandlerInitSuccess(t *testing.T) {
	sessions := userhttp.NewSessionManager("test-secret", false)
	store := &fakeDirectUploadStore{}
	handler := NewHandler(store, 10<<20)
	handler.SetUserAuthenticator(sessions.Authenticate)
	handler.SetPluginExists(func(pluginID string) bool { return pluginID == "demo" })
	handler.SetPluginAllowsFiles(func(pluginID string) bool { return pluginID == "demo" })

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := authenticatedRequest(t, sessions, http.MethodPost, "/api/files/init", bytes.NewBufferString(`{
		"plugin_id":"demo",
		"name":"report.pdf",
		"mime_type":"application/pdf",
		"size":123,
		"file_type":"document"
	}`))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if store.lastCreateMeta.ScopePluginID != "demo" {
		t.Fatalf("scope_plugin_id = %q, want demo", store.lastCreateMeta.ScopePluginID)
	}
	if store.lastCreateMeta.OwnerKind != filestore.FileOwnerUser || store.lastCreateMeta.OwnerUserID != 42 {
		t.Fatalf("unexpected owner metadata: %#v", store.lastCreateMeta)
	}
	if store.lastCreateMeta.State != filestore.FileStatePending {
		t.Fatalf("state = %q, want pending", store.lastCreateMeta.State)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["file_id"] != "file-1" {
		t.Fatalf("file_id = %#v, want file-1", body["file_id"])
	}
}

func TestHandlerInitSupportsUserToken(t *testing.T) {
	store := &fakeDirectUploadStore{}
	handler := NewHandler(store, 10<<20)
	handler.SetUserTokenAuthenticator(func(_ context.Context, rawToken string) (model.GlobalUserID, bool, error) {
		if rawToken != "sbuk_demo.secret" {
			return 0, false, nil
		}
		return 99, true, nil
	})
	handler.SetPluginExists(func(pluginID string) bool { return pluginID == "demo" })
	handler.SetPluginAllowsFiles(func(pluginID string) bool { return pluginID == "demo" })

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/files/init", bytes.NewBufferString(`{
		"plugin_id":"demo",
		"name":"report.pdf",
		"mime_type":"application/pdf",
		"size":123
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sbuk_demo.secret")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if store.lastCreateMeta.OwnerUserID != 99 {
		t.Fatalf("owner_user_id = %d, want 99", store.lastCreateMeta.OwnerUserID)
	}
}

func TestHandlerCompleteRejectsWrongOwner(t *testing.T) {
	sessions := userhttp.NewSessionManager("test-secret", false)
	store := &fakeDirectUploadStore{
		meta: map[string]filestore.FileMeta{
			"file-1": {
				ID:            "file-1",
				Name:          "report.pdf",
				MIMEType:      "application/pdf",
				FileType:      model.FileTypeDocument,
				State:         filestore.FileStatePending,
				ScopePluginID: "demo",
				OwnerKind:     filestore.FileOwnerUser,
				OwnerUserID:   7,
				ExpectedSize:  123,
			},
		},
	}
	handler := NewHandler(store, 10<<20)
	handler.SetUserAuthenticator(sessions.Authenticate)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := authenticatedRequest(t, sessions, http.MethodPost, "/api/files/file-1/complete", bytes.NewBuffer(nil))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestHandlerCompleteDeletesOversizedUpload(t *testing.T) {
	sessions := userhttp.NewSessionManager("test-secret", false)
	store := &fakeDirectUploadStore{
		meta: map[string]filestore.FileMeta{
			"file-1": {
				ID:            "file-1",
				Name:          "report.pdf",
				MIMEType:      "application/pdf",
				FileType:      model.FileTypeDocument,
				State:         filestore.FileStatePending,
				ScopePluginID: "demo",
				OwnerKind:     filestore.FileOwnerUser,
				OwnerUserID:   42,
				ExpectedSize:  128,
			},
		},
		completeRef: model.FileRef{
			ID:       "file-1",
			Name:     "report.pdf",
			MIMEType: "application/pdf",
			Size:     256,
			FileType: model.FileTypeDocument,
		},
	}
	handler := NewHandler(store, 200)
	handler.SetUserAuthenticator(sessions.Authenticate)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := authenticatedRequest(t, sessions, http.MethodPost, "/api/files/file-1/complete", bytes.NewBuffer(nil))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
	if store.lastDeletedID != "file-1" {
		t.Fatalf("deleted file = %q, want file-1", store.lastDeletedID)
	}
}

func TestHandlerDeleteSuccess(t *testing.T) {
	sessions := userhttp.NewSessionManager("test-secret", false)
	store := &fakeDirectUploadStore{
		meta: map[string]filestore.FileMeta{
			"file-1": {
				ID:          "file-1",
				Name:        "report.pdf",
				MIMEType:    "application/pdf",
				FileType:    model.FileTypeDocument,
				State:       filestore.FileStateReady,
				OwnerKind:   filestore.FileOwnerUser,
				OwnerUserID: 42,
			},
		},
	}
	handler := NewHandler(store, 10<<20)
	handler.SetUserAuthenticator(sessions.Authenticate)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := authenticatedRequest(t, sessions, http.MethodDelete, "/api/files/file-1", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if store.lastDeletedID != "file-1" {
		t.Fatalf("deleted file = %q, want file-1", store.lastDeletedID)
	}
}

type fakeDirectUploadStore struct {
	meta           map[string]filestore.FileMeta
	lastCreateMeta filestore.FileMeta
	lastDeletedID  string
	completeRef    model.FileRef
}

func (s *fakeDirectUploadStore) CreateDirectUpload(_ context.Context, meta filestore.FileMeta, expiry time.Duration) (filestore.DirectUpload, error) {
	s.lastCreateMeta = meta
	if s.meta == nil {
		s.meta = make(map[string]filestore.FileMeta)
	}
	meta.ID = "file-1"
	meta.ExpiresAt = ptrTime(time.Now().Add(expiry))
	s.meta[meta.ID] = meta
	return filestore.DirectUpload{
		FileID:    meta.ID,
		Method:    http.MethodPut,
		URL:       "https://upload.example.test/file-1",
		Headers:   map[string]string{"Content-Type": meta.MIMEType},
		ExpiresAt: *meta.ExpiresAt,
	}, nil
}

func (s *fakeDirectUploadStore) CompleteDirectUpload(_ context.Context, id string) (model.FileRef, error) {
	if s.completeRef.ID == "" {
		return model.FileRef{ID: id, Name: "report.pdf", MIMEType: "application/pdf", Size: 123, FileType: model.FileTypeDocument}, nil
	}
	return s.completeRef, nil
}

func (s *fakeDirectUploadStore) Store(context.Context, filestore.FileMeta, io.Reader) (model.FileRef, error) {
	return model.FileRef{}, fmt.Errorf("not implemented")
}

func (s *fakeDirectUploadStore) Get(context.Context, string) (io.ReadCloser, *filestore.FileMeta, error) {
	return nil, nil, fmt.Errorf("not implemented")
}

func (s *fakeDirectUploadStore) Meta(_ context.Context, id string) (*filestore.FileMeta, error) {
	meta, ok := s.meta[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	copy := meta
	return &copy, nil
}

func (s *fakeDirectUploadStore) Delete(_ context.Context, id string) error {
	s.lastDeletedID = id
	delete(s.meta, id)
	return nil
}

func (s *fakeDirectUploadStore) URL(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

func (s *fakeDirectUploadStore) Cleanup(context.Context) (int, error) {
	return 0, nil
}

func authenticatedRequest(t *testing.T, sessions *userhttp.SessionManager, method, target string, body *bytes.Buffer) *http.Request {
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

func ptrTime(t time.Time) *time.Time {
	return &t
}
