package hostapi

import (
	"fmt"
	"sync"
)

var DefaultRateLimits = map[string]int{
	"http_request":   20,
	"call_plugin":    10,
	"publish_event":  50,
	"kv_get":         200,
	"kv_set":         200,
	"kv_delete":      100,
	"kv_list":        50,
	"sql_open":       10,
	"sql_close":      10,
	"sql_exec":       100,
	"sql_query":      100,
	"sql_next":       5000,
	"sql_rows_close": 100,
	"sql_begin":      20,
	"sql_end":        20,
}

type RateLimiter struct {
	mu       sync.Mutex
	counts   map[string]int
	limits   map[string]int
	pluginID string
}

func NewRateLimiter(pluginID string, limits map[string]int) *RateLimiter {
	if limits == nil {
		limits = DefaultRateLimits
	}
	return &RateLimiter{
		counts:   make(map[string]int),
		limits:   limits,
		pluginID: pluginID,
	}
}

func (rl *RateLimiter) Allow(funcName string) error {
	limit, hasLimit := rl.limits[funcName]
	if !hasLimit {
		return nil
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.counts[funcName] >= limit {
		return fmt.Errorf("rate limit exceeded for %s: max %d calls per execution", funcName, limit)
	}
	rl.counts[funcName]++
	return nil
}

func (rl *RateLimiter) PluginID() string {
	return rl.pluginID
}

type rateLimiterKey struct{}
