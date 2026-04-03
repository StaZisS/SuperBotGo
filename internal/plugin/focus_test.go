package plugin

import (
	"testing"
	"time"

	"SuperBotGo/internal/model"
)

func TestFocusTracker_RecordAndLastPlugin(t *testing.T) {
	t.Parallel()

	ft := NewFocusTracker(time.Minute)

	var userID model.GlobalUserID = 42
	ft.Record(userID, "pluginA")

	got := ft.LastPlugin(userID)
	if got != "pluginA" {
		t.Errorf("LastPlugin = %q, want %q", got, "pluginA")
	}
}

func TestFocusTracker_LastPlugin_NoRecord(t *testing.T) {
	t.Parallel()

	ft := NewFocusTracker(time.Minute)

	got := ft.LastPlugin(999)
	if got != "" {
		t.Errorf("LastPlugin for unrecorded user = %q, want empty string", got)
	}
}

func TestFocusTracker_TTLExpiry(t *testing.T) {
	t.Parallel()

	const ttl = 50 * time.Millisecond
	ft := NewFocusTracker(ttl)

	var userID model.GlobalUserID = 1
	ft.Record(userID, "pluginX")

	// Verify it's available immediately.
	if got := ft.LastPlugin(userID); got != "pluginX" {
		t.Fatalf("expected pluginX before expiry, got %q", got)
	}

	// Wait for TTL to expire.
	time.Sleep(ttl + 20*time.Millisecond)

	got := ft.LastPlugin(userID)
	if got != "" {
		t.Errorf("expected empty string after TTL expiry, got %q", got)
	}
}

func TestFocusTracker_Overwrite(t *testing.T) {
	t.Parallel()

	ft := NewFocusTracker(time.Minute)

	var userID model.GlobalUserID = 7
	ft.Record(userID, "first")
	ft.Record(userID, "second")

	got := ft.LastPlugin(userID)
	if got != "second" {
		t.Errorf("LastPlugin after overwrite = %q, want %q", got, "second")
	}
}

func TestFocusTracker_DifferentUsersIndependent(t *testing.T) {
	t.Parallel()

	ft := NewFocusTracker(time.Minute)

	var user1 model.GlobalUserID = 10
	var user2 model.GlobalUserID = 20

	ft.Record(user1, "alpha")
	ft.Record(user2, "beta")

	if got := ft.LastPlugin(user1); got != "alpha" {
		t.Errorf("user1: got %q, want %q", got, "alpha")
	}
	if got := ft.LastPlugin(user2); got != "beta" {
		t.Errorf("user2: got %q, want %q", got, "beta")
	}
}

func TestFocusTracker_ExpiredEntryCleanup(t *testing.T) {
	t.Parallel()

	const ttl = 50 * time.Millisecond
	ft := NewFocusTracker(ttl)

	var userID model.GlobalUserID = 55

	// Record an entry and let it expire.
	ft.Record(userID, "old")
	time.Sleep(ttl + 20*time.Millisecond)

	// Accessing the expired entry should return "" and clean it from the map.
	if got := ft.LastPlugin(userID); got != "" {
		t.Fatalf("expected empty after expiry, got %q", got)
	}

	// Record a new entry for the same user — should work without interference.
	ft.Record(userID, "new")
	if got := ft.LastPlugin(userID); got != "new" {
		t.Errorf("expected %q after re-record, got %q", "new", got)
	}
}
