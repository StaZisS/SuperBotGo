package hostapi

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestKVStore_BasicSetGet(t *testing.T) {
	store := NewKVStore()

	// Set and get a value.
	if err := store.Set("plugin1", "key1", "value1", 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, found, err := store.Get("plugin1", "key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("expected key to be found")
	}
	if val != "value1" {
		t.Fatalf("expected value1, got %q", val)
	}
}

func TestKVStore_NotFound(t *testing.T) {
	store := NewKVStore()

	val, found, err := store.Get("plugin1", "nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if found {
		t.Fatal("expected key not to be found")
	}
	if val != "" {
		t.Fatalf("expected empty string, got %q", val)
	}
}

func TestKVStore_Overwrite(t *testing.T) {
	store := NewKVStore()

	store.Set("p1", "k", "v1", 0)
	store.Set("p1", "k", "v2", 0)

	val, found, _ := store.Get("p1", "k")
	if !found || val != "v2" {
		t.Fatalf("expected v2, got %q (found=%v)", val, found)
	}
}

func TestKVStore_PluginIsolation(t *testing.T) {
	store := NewKVStore()

	store.Set("pluginA", "shared_key", "A_value", 0)
	store.Set("pluginB", "shared_key", "B_value", 0)

	valA, _, _ := store.Get("pluginA", "shared_key")
	valB, _, _ := store.Get("pluginB", "shared_key")

	if valA != "A_value" {
		t.Fatalf("pluginA expected A_value, got %q", valA)
	}
	if valB != "B_value" {
		t.Fatalf("pluginB expected B_value, got %q", valB)
	}
}

func TestKVStore_Delete(t *testing.T) {
	store := NewKVStore()

	store.Set("p1", "k", "v", 0)
	store.Delete("p1", "k")

	_, found, _ := store.Get("p1", "k")
	if found {
		t.Fatal("expected key to be deleted")
	}
}

func TestKVStore_DeleteNonexistent(t *testing.T) {
	store := NewKVStore()

	// Should not error.
	if err := store.Delete("p1", "nonexistent"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestKVStore_List(t *testing.T) {
	store := NewKVStore()

	store.Set("p1", "user:1", "a", 0)
	store.Set("p1", "user:2", "b", 0)
	store.Set("p1", "session:1", "c", 0)

	// List with prefix.
	keys, err := store.List("p1", "user:")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d: %v", len(keys), keys)
	}

	// List all.
	keys, err = store.List("p1", "")
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
}

func TestKVStore_TTLExpiry(t *testing.T) {
	store := NewKVStore()

	// Set with a very short TTL.
	store.Set("p1", "ephemeral", "val", 1*time.Millisecond)

	// Wait for expiry.
	time.Sleep(5 * time.Millisecond)

	_, found, _ := store.Get("p1", "ephemeral")
	if found {
		t.Fatal("expected key to have expired")
	}
}

func TestKVStore_TTLNotExpired(t *testing.T) {
	store := NewKVStore()

	store.Set("p1", "persistent", "val", 10*time.Second)

	val, found, _ := store.Get("p1", "persistent")
	if !found {
		t.Fatal("expected key to be found (not expired yet)")
	}
	if val != "val" {
		t.Fatalf("expected val, got %q", val)
	}
}

func TestKVStore_MaxKeysLimit(t *testing.T) {
	store := NewKVStore()

	// Fill up to the limit.
	for i := 0; i < kvMaxKeysPerPlugin; i++ {
		key := fmt.Sprintf("key_%d", i)
		if err := store.Set("p1", key, "v", 0); err != nil {
			t.Fatalf("Set key %d: %v", i, err)
		}
	}

	// One more should fail.
	err := store.Set("p1", "key_overflow", "v", 0)
	if err == nil {
		t.Fatal("expected error for exceeding max keys")
	}
	if !strings.Contains(err.Error(), "too many keys") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKVStore_MaxValueSize(t *testing.T) {
	store := NewKVStore()

	bigValue := strings.Repeat("x", kvMaxValueSize+1)
	err := store.Set("p1", "big", bigValue, 0)
	if err == nil {
		t.Fatal("expected error for oversized value")
	}
	if !strings.Contains(err.Error(), "value too large") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKVStore_MaxTotalSize(t *testing.T) {
	store := NewKVStore()

	// Each value is ~64KB (just under the per-value limit).
	valueSize := kvMaxValueSize
	value := strings.Repeat("x", valueSize)

	// 10 MB / 64 KB = 160 entries needed to exceed total.
	// We should be able to set around 159 before hitting the limit.
	var lastErr error
	for i := 0; i < 200; i++ {
		key := fmt.Sprintf("k%d", i)
		if err := store.Set("p1", key, value, 0); err != nil {
			lastErr = err
			break
		}
	}

	if lastErr == nil {
		t.Fatal("expected error for exceeding total storage")
	}
	if !strings.Contains(lastErr.Error(), "total storage exceeded") {
		t.Fatalf("unexpected error: %v", lastErr)
	}
}

func TestKVStore_DropPlugin(t *testing.T) {
	store := NewKVStore()

	store.Set("p1", "k1", "v1", 0)
	store.Set("p1", "k2", "v2", 0)

	store.DropPlugin("p1")

	_, found, _ := store.Get("p1", "k1")
	if found {
		t.Fatal("expected key to be gone after DropPlugin")
	}
}

func TestKVStore_ConcurrentAccess(t *testing.T) {
	store := NewKVStore()
	pluginID := "concurrent_plugin"

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key_%d", i%10)
			value := fmt.Sprintf("value_%d", i)
			store.Set(pluginID, key, value, 0)
			store.Get(pluginID, key)
			store.List(pluginID, "key_")
		}(i)
	}
	wg.Wait()

	// Just verify we didn't panic — if we got here, concurrency is safe.
	keys, _ := store.List(pluginID, "")
	if len(keys) == 0 {
		t.Fatal("expected some keys after concurrent access")
	}
}
