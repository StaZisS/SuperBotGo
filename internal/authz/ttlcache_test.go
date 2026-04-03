package authz

import (
	"sync"
	"testing"
	"time"
)

func TestTTLCache_SetAndGet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		key       string
		value     int
		wantValue int
		wantOK    bool
	}{
		{
			name:      "existing key returns value and true",
			key:       "a",
			value:     42,
			wantValue: 42,
			wantOK:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := NewTTLCache[string, int](time.Minute)
			c.Set(tt.key, tt.value)

			got, ok := c.Get(tt.key)
			if ok != tt.wantOK {
				t.Errorf("Get() ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.wantValue {
				t.Errorf("Get() value = %v, want %v", got, tt.wantValue)
			}
		})
	}
}

func TestTTLCache_GetMissing(t *testing.T) {
	t.Parallel()

	c := NewTTLCache[string, int](time.Minute)

	got, ok := c.Get("nonexistent")
	if ok {
		t.Error("Get() ok = true for missing key, want false")
	}
	if got != 0 {
		t.Errorf("Get() value = %v, want zero value 0", got)
	}
}

func TestTTLCache_Expiry(t *testing.T) {
	t.Parallel()

	const ttl = 50 * time.Millisecond
	c := NewTTLCache[string, string](ttl)
	c.Set("key", "value")

	// Value should be available before expiry.
	if v, ok := c.Get("key"); !ok || v != "value" {
		t.Fatalf("Get() before expiry: got (%q, %v), want (%q, true)", v, ok, "value")
	}

	time.Sleep(ttl + 20*time.Millisecond)

	got, ok := c.Get("key")
	if ok {
		t.Errorf("Get() ok = true after TTL expiry, want false")
	}
	if got != "" {
		t.Errorf("Get() value = %q after TTL expiry, want zero value", got)
	}
}

func TestTTLCache_Delete(t *testing.T) {
	t.Parallel()

	c := NewTTLCache[string, int](time.Minute)
	c.Set("key", 99)
	c.Delete("key")

	_, ok := c.Get("key")
	if ok {
		t.Error("Get() ok = true after Delete, want false")
	}
}

func TestTTLCache_Clear(t *testing.T) {
	t.Parallel()

	c := NewTTLCache[string, int](time.Minute)
	keys := []string{"a", "b", "c"}
	for i, k := range keys {
		c.Set(k, i)
	}

	c.Clear()

	for _, k := range keys {
		if _, ok := c.Get(k); ok {
			t.Errorf("Get(%q) ok = true after Clear, want false", k)
		}
	}
}

func TestTTLCache_Overwrite(t *testing.T) {
	t.Parallel()

	c := NewTTLCache[string, string](time.Minute)
	c.Set("key", "first")
	c.Set("key", "second")

	got, ok := c.Get("key")
	if !ok {
		t.Fatal("Get() ok = false after overwrite, want true")
	}
	if got != "second" {
		t.Errorf("Get() value = %q, want %q", got, "second")
	}
}

func TestTTLCache_ExpiredEntryIsDeletedFromMap(t *testing.T) {
	t.Parallel()

	const ttl = 50 * time.Millisecond
	c := NewTTLCache[string, int](ttl)
	c.Set("key", 1)

	time.Sleep(ttl + 20*time.Millisecond)

	// Trigger the lazy deletion via Get.
	_, _ = c.Get("key")

	// Verify the entry was removed from the internal map.
	c.mu.RLock()
	_, exists := c.entries["key"]
	c.mu.RUnlock()

	if exists {
		t.Error("expired entry still present in internal map after Get")
	}
}

func TestTTLCache_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	c := NewTTLCache[int, int](time.Minute)
	const goroutines = 50
	const ops = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := range goroutines {
		go func(id int) {
			defer wg.Done()
			for i := range ops {
				key := (id*ops + i) % 20 // overlap keys to increase contention
				c.Set(key, i)
				c.Get(key)
				if i%10 == 0 {
					c.Delete(key)
				}
			}
		}(g)
	}

	wg.Wait()
	// If we reach here without a race/panic the test passes.
}
