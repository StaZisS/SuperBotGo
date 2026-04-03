package authz

import (
	"sync"
	"time"
)

type ttlEntry[V any] struct {
	value     V
	expiresAt time.Time
}

type TTLCache[K comparable, V any] struct {
	mu      sync.RWMutex
	entries map[K]ttlEntry[V]
	ttl     time.Duration
}

func NewTTLCache[K comparable, V any](ttl time.Duration) *TTLCache[K, V] {
	return &TTLCache[K, V]{
		entries: make(map[K]ttlEntry[V]),
		ttl:     ttl,
	}
}

func (c *TTLCache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		var zero V
		return zero, false
	}
	if time.Now().After(e.expiresAt) {
		c.mu.Lock()
		// Re-check under write lock to avoid deleting a fresh entry.
		if e2, ok2 := c.entries[key]; ok2 && time.Now().After(e2.expiresAt) {
			delete(c.entries, key)
		}
		c.mu.Unlock()
		var zero V
		return zero, false
	}
	return e.value, true
}

func (c *TTLCache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	c.entries[key] = ttlEntry[V]{value: value, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *TTLCache[K, V]) Delete(key K) {
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}

func (c *TTLCache[K, V]) Clear() {
	c.mu.Lock()
	c.entries = make(map[K]ttlEntry[V])
	c.mu.Unlock()
}
