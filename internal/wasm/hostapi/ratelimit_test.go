package hostapi

import (
	"context"
	"strings"
	"testing"
)

func TestRateLimiter_AllowWithinLimit(t *testing.T) {
	rl := NewRateLimiter("test-plugin", map[string]int{
		"db_query": 3,
	})

	for i := 0; i < 3; i++ {
		if err := rl.Allow("db_query"); err != nil {
			t.Fatalf("call %d: unexpected error: %v", i+1, err)
		}
	}
}

func TestRateLimiter_AllowExceedsLimit(t *testing.T) {
	rl := NewRateLimiter("test-plugin", map[string]int{
		"db_query": 2,
	})

	// Use up the allowance.
	for i := 0; i < 2; i++ {
		if err := rl.Allow("db_query"); err != nil {
			t.Fatalf("call %d: unexpected error: %v", i+1, err)
		}
	}

	// Third call should be rejected.
	err := rl.Allow("db_query")
	if err == nil {
		t.Fatal("expected rate limit error, got nil")
	}
	if !strings.Contains(err.Error(), "rate limit exceeded for db_query") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "max 2 calls per execution") {
		t.Fatalf("error message missing limit value: %v", err)
	}
}

func TestRateLimiter_NoLimitConfigured(t *testing.T) {
	rl := NewRateLimiter("test-plugin", map[string]int{
		"db_query": 5,
	})

	// "some_other_func" has no limit configured, should always be allowed.
	for i := 0; i < 100; i++ {
		if err := rl.Allow("some_other_func"); err != nil {
			t.Fatalf("call %d: unexpected error for unconfigured function: %v", i+1, err)
		}
	}
}

func TestRateLimiter_DefaultLimits(t *testing.T) {
	rl := NewRateLimiter("test-plugin", nil)

	// Should use DefaultRateLimits.
	for funcName, limit := range DefaultRateLimits {
		for i := 0; i < limit; i++ {
			if err := rl.Allow(funcName); err != nil {
				t.Fatalf("%s call %d: unexpected error: %v", funcName, i+1, err)
			}
		}
		err := rl.Allow(funcName)
		if err == nil {
			t.Fatalf("%s: expected rate limit error after %d calls, got nil", funcName, limit)
		}
	}
}

func TestRateLimiter_IndependentCounters(t *testing.T) {
	rl := NewRateLimiter("test-plugin", map[string]int{
		"db_query":     2,
		"http_request": 1,
	})

	// Use up db_query.
	for i := 0; i < 2; i++ {
		if err := rl.Allow("db_query"); err != nil {
			t.Fatalf("db_query call %d: %v", i+1, err)
		}
	}

	// http_request should still work independently.
	if err := rl.Allow("http_request"); err != nil {
		t.Fatalf("http_request call 1: %v", err)
	}

	// db_query is exhausted.
	if err := rl.Allow("db_query"); err == nil {
		t.Fatal("expected db_query to be rate limited")
	}

	// http_request is now exhausted too.
	if err := rl.Allow("http_request"); err == nil {
		t.Fatal("expected http_request to be rate limited")
	}
}

func TestRateLimiter_PluginID(t *testing.T) {
	rl := NewRateLimiter("my-plugin", nil)
	if got := rl.PluginID(); got != "my-plugin" {
		t.Fatalf("PluginID() = %q, want %q", got, "my-plugin")
	}
}

func TestContextWithRateLimiter(t *testing.T) {
	h := NewHostAPI(Dependencies{})
	ctx := context.Background()

	enriched := h.ContextWithRateLimiter(ctx, "ctx-plugin")

	rl, ok := enriched.Value(rateLimiterKey{}).(*RateLimiter)
	if !ok || rl == nil {
		t.Fatal("expected RateLimiter in context")
	}
	if rl.PluginID() != "ctx-plugin" {
		t.Fatalf("PluginID() = %q, want %q", rl.PluginID(), "ctx-plugin")
	}
}

func TestSetRateLimits_CustomLimits(t *testing.T) {
	h := NewHostAPI(Dependencies{})
	custom := map[string]int{
		"db_query":     5,
		"http_request": 2,
	}
	h.SetRateLimits(custom)

	rl := h.NewRateLimiterForPlugin("custom-plugin")

	// db_query should allow exactly 5 calls.
	for i := 0; i < 5; i++ {
		if err := rl.Allow("db_query"); err != nil {
			t.Fatalf("db_query call %d: %v", i+1, err)
		}
	}
	if err := rl.Allow("db_query"); err == nil {
		t.Fatal("expected db_query to be rate limited after 5 calls")
	}

	// http_request should allow exactly 2 calls.
	for i := 0; i < 2; i++ {
		if err := rl.Allow("http_request"); err != nil {
			t.Fatalf("http_request call %d: %v", i+1, err)
		}
	}
	if err := rl.Allow("http_request"); err == nil {
		t.Fatal("expected http_request to be rate limited after 2 calls")
	}
}
