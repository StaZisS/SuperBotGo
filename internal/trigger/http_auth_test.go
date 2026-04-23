package trigger

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"SuperBotGo/internal/model"
)

func TestHTTPTriggerResolveSetting_Defaults(t *testing.T) {
	h := NewHTTPTriggerHandler(nil, nil)

	setting, err := h.resolveSetting(t.Context(), "demo", "incoming")
	if err != nil {
		t.Fatalf("resolveSetting() error = %v", err)
	}
	if !setting.Enabled {
		t.Fatal("expected setting.Enabled to default to true")
	}
	if !setting.AllowUserKeys {
		t.Fatal("expected setting.AllowUserKeys to default to true")
	}
	if setting.AllowServiceKeys {
		t.Fatal("expected setting.AllowServiceKeys to default to false")
	}
}

func TestHTTPTriggerResolvePrincipal_UserSession(t *testing.T) {
	h := NewHTTPTriggerHandler(nil, nil)
	h.SetUserAuthenticator(func(_ *http.Request) (model.GlobalUserID, bool) {
		return 42, true
	})
	h.SetPolicyEvaluator(func(_ context.Context, expression string, userID model.GlobalUserID) (bool, error) {
		if expression != "user.id == 42" {
			t.Fatalf("unexpected expression %q", expression)
		}
		if userID != 42 {
			t.Fatalf("unexpected userID %d", userID)
		}
		return true, nil
	})

	req := httptest.NewRequest("GET", "/api/triggers/http/demo/incoming", nil)
	principal, status, err := h.resolvePrincipal(req, "demo", "incoming", HTTPTriggerSetting{
		Enabled:          true,
		AllowUserKeys:    true,
		AllowServiceKeys: false,
		PolicyExpression: "user.id == 42",
	})
	if err != nil {
		t.Fatalf("resolvePrincipal() error = %v", err)
	}
	if status != 0 {
		t.Fatalf("resolvePrincipal() status = %d, want 0", status)
	}
	if principal.authData == nil || principal.authData.Kind != model.HTTPAuthUser || principal.authData.UserID != 42 {
		t.Fatalf("unexpected principal auth data: %#v", principal.authData)
	}
}

func TestHTTPTriggerResolvePrincipal_ServiceKey(t *testing.T) {
	h := NewHTTPTriggerHandler(nil, nil)
	h.SetServiceAuthenticator(func(_ context.Context, rawToken, pluginID, triggerName string) (ServiceKeyPrincipal, bool, error) {
		if rawToken != "sbsk_demo.secret" || pluginID != "demo" || triggerName != "incoming" {
			t.Fatalf("unexpected service auth input token=%q plugin=%q trigger=%q", rawToken, pluginID, triggerName)
		}
		return ServiceKeyPrincipal{ID: 7}, true, nil
	})

	req := httptest.NewRequest("GET", "/api/triggers/http/demo/incoming", nil)
	req.Header.Set("Authorization", "Bearer sbsk_demo.secret")

	principal, status, err := h.resolvePrincipal(req, "demo", "incoming", HTTPTriggerSetting{
		Enabled:          true,
		AllowUserKeys:    false,
		AllowServiceKeys: true,
	})
	if err != nil {
		t.Fatalf("resolvePrincipal() error = %v", err)
	}
	if status != 0 {
		t.Fatalf("resolvePrincipal() status = %d, want 0", status)
	}
	if principal.authData == nil || principal.authData.Kind != model.HTTPAuthService || principal.authData.ServiceKeyID != 7 {
		t.Fatalf("unexpected principal auth data: %#v", principal.authData)
	}
}

func TestHTTPTriggerResolvePrincipal_UserBearerToken(t *testing.T) {
	h := NewHTTPTriggerHandler(nil, nil)
	h.SetUserTokenAuthenticator(func(_ context.Context, rawToken string) (model.GlobalUserID, bool, error) {
		if rawToken != "sbuk_demo.secret" {
			t.Fatalf("unexpected user token %q", rawToken)
		}
		return 55, true, nil
	})
	h.SetPolicyEvaluator(func(_ context.Context, expression string, userID model.GlobalUserID) (bool, error) {
		if expression != "user.id == 55" {
			t.Fatalf("unexpected expression %q", expression)
		}
		if userID != 55 {
			t.Fatalf("unexpected userID %d", userID)
		}
		return true, nil
	})

	req := httptest.NewRequest("GET", "/api/triggers/http/demo/incoming", nil)
	req.Header.Set("Authorization", "Bearer sbuk_demo.secret")

	principal, status, err := h.resolvePrincipal(req, "demo", "incoming", HTTPTriggerSetting{
		Enabled:          true,
		AllowUserKeys:    true,
		AllowServiceKeys: false,
		PolicyExpression: "user.id == 55",
	})
	if err != nil {
		t.Fatalf("resolvePrincipal() error = %v", err)
	}
	if status != 0 {
		t.Fatalf("resolvePrincipal() status = %d, want 0", status)
	}
	if principal.authData == nil || principal.authData.Kind != model.HTTPAuthUser || principal.authData.UserID != 55 {
		t.Fatalf("unexpected principal auth data: %#v", principal.authData)
	}
}

func TestHTTPTriggerResolvePrincipal_DenyUserWhenPublicAccessNotAllowed(t *testing.T) {
	h := NewHTTPTriggerHandler(nil, nil)
	h.SetUserAuthenticator(func(_ *http.Request) (model.GlobalUserID, bool) {
		return 99, true
	})

	req := httptest.NewRequest("GET", "/api/triggers/http/demo/incoming", nil)
	_, status, err := h.resolvePrincipal(req, "demo", "incoming", HTTPTriggerSetting{
		Enabled:          true,
		AllowUserKeys:    false,
		AllowServiceKeys: true,
	})
	if err == nil {
		t.Fatal("expected resolvePrincipal() to deny disabled user access")
	}
	if status != 403 {
		t.Fatalf("resolvePrincipal() status = %d, want 403", status)
	}
}

func TestHTTPTriggerResolvePrincipal_RequiresAuthentication(t *testing.T) {
	h := NewHTTPTriggerHandler(nil, nil)

	req := httptest.NewRequest("GET", "/api/triggers/http/demo/incoming", nil)
	_, status, err := h.resolvePrincipal(req, "demo", "incoming", HTTPTriggerSetting{
		Enabled:          true,
		AllowUserKeys:    true,
		AllowServiceKeys: false,
	})
	if err == nil {
		t.Fatal("expected resolvePrincipal() to require authentication")
	}
	if status != 401 {
		t.Fatalf("resolvePrincipal() status = %d, want 401", status)
	}
}

func TestHTTPTriggerResolvePrincipal_AllowsAnonymousWhenAllAuthModesDisabled(t *testing.T) {
	h := NewHTTPTriggerHandler(nil, nil)
	h.SetUserAuthenticator(func(_ *http.Request) (model.GlobalUserID, bool) {
		return 42, true
	})

	req := httptest.NewRequest("GET", "/api/triggers/http/demo/incoming", nil)
	principal, status, err := h.resolvePrincipal(req, "demo", "incoming", HTTPTriggerSetting{
		Enabled:          true,
		AllowUserKeys:    false,
		AllowServiceKeys: false,
	})
	if err != nil {
		t.Fatalf("resolvePrincipal() error = %v", err)
	}
	if status != 0 {
		t.Fatalf("resolvePrincipal() status = %d, want 0", status)
	}
	if principal.authData != nil {
		t.Fatalf("expected anonymous principal, got %#v", principal.authData)
	}
}

func TestHTTPTriggerResolvePrincipal_InvalidBearerDoesNotFallbackToSession(t *testing.T) {
	h := NewHTTPTriggerHandler(nil, nil)
	h.SetUserTokenAuthenticator(func(_ context.Context, rawToken string) (model.GlobalUserID, bool, error) {
		if rawToken != "sbuk_invalid.secret" {
			t.Fatalf("unexpected user token %q", rawToken)
		}
		return 0, false, nil
	})
	h.SetUserAuthenticator(func(_ *http.Request) (model.GlobalUserID, bool) {
		return 42, true
	})

	req := httptest.NewRequest("GET", "/api/triggers/http/demo/incoming", nil)
	req.Header.Set("Authorization", "Bearer sbuk_invalid.secret")

	_, status, err := h.resolvePrincipal(req, "demo", "incoming", HTTPTriggerSetting{
		Enabled:          true,
		AllowUserKeys:    true,
		AllowServiceKeys: false,
	})
	if err == nil {
		t.Fatal("expected resolvePrincipal() to reject invalid bearer token")
	}
	if status != http.StatusUnauthorized {
		t.Fatalf("resolvePrincipal() status = %d, want %d", status, http.StatusUnauthorized)
	}
}

func TestLoginRedirectURL_ForBrowserHTMLRequest(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/triggers/http/demo/portal?tab=debug", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	redirectURL, ok := loginRedirectURL(req, http.StatusUnauthorized, HTTPTriggerSetting{
		Enabled:          true,
		AllowUserKeys:    true,
		AllowServiceKeys: false,
	})
	if !ok {
		t.Fatal("expected loginRedirectURL() to produce redirect")
	}

	want := "/api/auth/tsu/start?return_to=%2Fapi%2Ftriggers%2Fhttp%2Fdemo%2Fportal%3Ftab%3Ddebug"
	if redirectURL != want {
		t.Fatalf("loginRedirectURL() = %q, want %q", redirectURL, want)
	}
}

func TestLoginRedirectURL_SkipsAPIRequests(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/triggers/http/schedule/api/schedule", nil)
	req.Header.Set("Accept", "application/json")

	redirectURL, ok := loginRedirectURL(req, http.StatusUnauthorized, HTTPTriggerSetting{
		Enabled:          true,
		AllowUserKeys:    true,
		AllowServiceKeys: false,
	})
	if ok || redirectURL != "" {
		t.Fatalf("loginRedirectURL() = %q, %v; want no redirect", redirectURL, ok)
	}
}

func TestLoginRedirectURL_SkipsServiceTokenRequests(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/triggers/http/demo/portal", nil)
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Authorization", "Bearer sbsk_demo.secret")

	redirectURL, ok := loginRedirectURL(req, http.StatusUnauthorized, HTTPTriggerSetting{
		Enabled:          true,
		AllowUserKeys:    true,
		AllowServiceKeys: true,
	})
	if ok || redirectURL != "" {
		t.Fatalf("loginRedirectURL() = %q, %v; want no redirect", redirectURL, ok)
	}
}
