package hostapi

import (
	"encoding/json"
	"strings"
	"testing"

	wasmrt "SuperBotGo/internal/wasm/runtime"
)

func TestResolveHTTPPolicies(t *testing.T) {
	requirements := []wasmrt.RequirementDef{
		{Type: "http"},
		{Type: "http", Name: "github"},
	}
	config := json.RawMessage(`{
		"requirements": {
			"http": {
				"default": {
					"allowed_hosts": ["api.example.com"],
					"allowed_methods": ["get"],
					"max_request_body_bytes": 32,
					"max_response_body_bytes": 64
				},
				"github": {
					"allowed_hosts": ["api.github.com"],
					"allowed_methods": ["POST"]
				}
			}
		}
	}`)

	policies, err := ResolveHTTPPolicies(requirements, config)
	if err != nil {
		t.Fatalf("ResolveHTTPPolicies() error = %v", err)
	}

	if got := policies["default"].AllowedMethods[0]; got != "GET" {
		t.Fatalf("default allowed method = %q, want GET", got)
	}
	if got := policies["github"].AllowedHosts[0]; got != "api.github.com" {
		t.Fatalf("github allowed host = %q", got)
	}
	if got := policies["default"].MaxResponseBodyBytes; got != 64 {
		t.Fatalf("default max response size = %d, want 64", got)
	}
}

func TestEnforceHTTPRequestPolicy(t *testing.T) {
	policy := HTTPPolicy{
		Requirement:          "default",
		AllowedHosts:         []string{"api.example.com", "*.example.org"},
		AllowedMethods:       []string{"GET", "POST"},
		MaxRequestBodyBytes:  8,
		MaxResponseBodyBytes: 16,
	}

	tests := []struct {
		name    string
		method  string
		rawURL  string
		bodyLen int64
		wantErr string
	}{
		{name: "allowed exact host", method: "GET", rawURL: "https://api.example.com/users"},
		{name: "allowed wildcard host", method: "POST", rawURL: "https://sub.example.org/users"},
		{name: "denied host", method: "GET", rawURL: "https://api.blocked.com", wantErr: "does not allow host"},
		{name: "denied method", method: "DELETE", rawURL: "https://api.example.com", wantErr: "does not allow method"},
		{name: "request body too large", method: "POST", rawURL: "https://api.example.com", bodyLen: 9, wantErr: "body exceeds limit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := enforceHTTPRequestPolicy(policy, tt.method, tt.rawURL, tt.bodyLen)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("enforceHTTPRequestPolicy() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestReadHTTPResponseBody(t *testing.T) {
	body, err := readHTTPResponseBody(strings.NewReader("hello"), 5)
	if err != nil {
		t.Fatalf("readHTTPResponseBody() error = %v", err)
	}
	if string(body) != "hello" {
		t.Fatalf("body = %q, want hello", string(body))
	}

	if _, err := readHTTPResponseBody(strings.NewReader("too-long"), 3); err == nil {
		t.Fatal("expected response size limit error")
	}
}
