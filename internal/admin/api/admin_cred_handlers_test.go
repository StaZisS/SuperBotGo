package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireSameAdminUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		auth           adminSessionAuthenticator
		targetUserID   int64
		wantStatusCode int
		wantAllowed    bool
	}{
		{
			name:           "missing session",
			auth:           stubAdminSessionAuth{ok: false},
			targetUserID:   10,
			wantStatusCode: http.StatusUnauthorized,
			wantAllowed:    false,
		},
		{
			name:           "other admin",
			auth:           stubAdminSessionAuth{userID: 15, ok: true},
			targetUserID:   10,
			wantStatusCode: http.StatusForbidden,
			wantAllowed:    false,
		},
		{
			name:           "same admin",
			auth:           stubAdminSessionAuth{userID: 10, ok: true},
			targetUserID:   10,
			wantStatusCode: 0,
			wantAllowed:    true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := &AdminCredHandler{auth: tt.auth}
			req := httptest.NewRequest(http.MethodDelete, "/api/admin/admins/10", nil)
			rec := httptest.NewRecorder()

			got := h.requireSameAdminUser(rec, req, tt.targetUserID)
			if got != tt.wantAllowed {
				t.Fatalf("expected allowed=%v, got %v", tt.wantAllowed, got)
			}
			if tt.wantStatusCode != 0 && rec.Code != tt.wantStatusCode {
				t.Fatalf("expected status %d, got %d", tt.wantStatusCode, rec.Code)
			}
		})
	}
}

type stubAdminSessionAuth struct {
	userID int64
	ok     bool
}

func (s stubAdminSessionAuth) AuthenticateSession(*http.Request) (int64, bool) {
	return s.userID, s.ok
}
