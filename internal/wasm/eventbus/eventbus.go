package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Handler func(ctx context.Context, topic string, payload []byte) error

type DLQEntry struct {
	ID         string          `json:"id"`
	Topic      string          `json:"topic"`
	Payload    json.RawMessage `json:"payload"`
	PluginID   string          `json:"plugin_id"`
	Error      string          `json:"error"`
	Timestamp  time.Time       `json:"timestamp"`
	RetryCount int             `json:"retry_count"`
}

type Config struct {
	MaxRetries int

	Backoff []time.Duration

	DLQMaxSize int
}

func (c *Config) withDefaults() Config {
	out := *c
	if out.MaxRetries <= 0 {
		out.MaxRetries = 3
	}
	if len(out.Backoff) == 0 {
		out.Backoff = []time.Duration{
			100 * time.Millisecond,
			500 * time.Millisecond,
			2 * time.Second,
		}
	}
	if out.DLQMaxSize <= 0 {
		out.DLQMaxSize = 1000
	}
	return out
}

type pluginIDKey struct{}

func ContextWithPluginID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, pluginIDKey{}, id)
}

func pluginIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(pluginIDKey{}).(string); ok {
		return id
	}
	return ""
}

type Metrics struct {
	PublishTotal *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	m := &Metrics{
		PublishTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "plugin_event_publish_total",
			Help: "Total number of plugin event publish attempts by topic and outcome",
		}, []string{"topic", "status"}),
	}
	prometheus.MustRegister(m.PublishTotal)
	return m
}

type Bus struct {
	cfg     Config
	metrics *Metrics

	mu       sync.RWMutex
	handlers []Handler

	dlqMu  sync.Mutex
	dlq    []DLQEntry
	dlqSeq uint64
}

func New(cfg *Config, m *Metrics) *Bus {
	var c Config
	if cfg != nil {
		c = cfg.withDefaults()
	} else {
		c = (&Config{}).withDefaults()
	}
	return &Bus{
		cfg:     c,
		metrics: m,
	}
}

func (b *Bus) Subscribe(h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = append(b.handlers, h)
}

func (b *Bus) Publish(ctx context.Context, topic string, payload []byte) error {
	b.mu.RLock()
	handlers := make([]Handler, len(b.handlers))
	copy(handlers, b.handlers)
	b.mu.RUnlock()

	if len(handlers) == 0 {
		b.incMetric(topic, "delivered")
		return nil
	}

	pluginID := pluginIDFromContext(ctx)

	var firstErr error
	for i, h := range handlers {
		if err := b.deliverWithRetry(ctx, h, topic, payload); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			b.addToDLQ(topic, payload, pluginID, err)
			b.incMetric(topic, "dead_lettered")
			slog.Error("event delivery exhausted retries, moved to DLQ",
				"topic", topic,
				"handler_index", i,
				"plugin_id", pluginID,
				"error", err,
			)
		} else {
			b.incMetric(topic, "delivered")
		}
	}

	return firstErr
}

func (b *Bus) deliverWithRetry(ctx context.Context, h Handler, topic string, payload []byte) error {
	var lastErr error
	maxAttempts := 1 + b.cfg.MaxRetries

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			delay := b.backoffDelay(attempt - 1)
			b.incMetric(topic, "retrying")
			slog.Info("retrying event delivery",
				"topic", topic,
				"attempt", attempt+1,
				"delay", delay,
			)
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during retry backoff: %w", ctx.Err())
			case <-time.After(delay):
			}
		}

		if err := h(ctx, topic, payload); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}

func (b *Bus) backoffDelay(retryIndex int) time.Duration {
	if retryIndex < len(b.cfg.Backoff) {
		return b.cfg.Backoff[retryIndex]
	}
	return b.cfg.Backoff[len(b.cfg.Backoff)-1]
}

func (b *Bus) addToDLQ(topic string, payload []byte, pluginID string, err error) {
	b.dlqMu.Lock()
	defer b.dlqMu.Unlock()

	b.dlqSeq++
	entry := DLQEntry{
		ID:         fmt.Sprintf("dlq-%d", b.dlqSeq),
		Topic:      topic,
		Payload:    json.RawMessage(payload),
		PluginID:   pluginID,
		Error:      err.Error(),
		Timestamp:  time.Now(),
		RetryCount: b.cfg.MaxRetries,
	}

	if len(b.dlq) >= b.cfg.DLQMaxSize {
		b.dlq = append(b.dlq[1:], entry)
	} else {
		b.dlq = append(b.dlq, entry)
	}
}

func (b *Bus) DLQEntries() []DLQEntry {
	b.dlqMu.Lock()
	defer b.dlqMu.Unlock()

	out := make([]DLQEntry, len(b.dlq))
	copy(out, b.dlq)
	return out
}

func (b *Bus) DLQLen() int {
	b.dlqMu.Lock()
	defer b.dlqMu.Unlock()
	return len(b.dlq)
}

func (b *Bus) ReplayDLQ(ctx context.Context) (replayed int, err error) {
	b.dlqMu.Lock()
	entries := make([]DLQEntry, len(b.dlq))
	copy(entries, b.dlq)
	b.dlqMu.Unlock()

	var remaining []DLQEntry
	var firstErr error

	for _, entry := range entries {
		replayCtx := ContextWithPluginID(ctx, entry.PluginID)
		if pubErr := b.Publish(replayCtx, entry.Topic, entry.Payload); pubErr != nil {
			entry.Error = pubErr.Error()
			entry.Timestamp = time.Now()
			entry.RetryCount += b.cfg.MaxRetries
			remaining = append(remaining, entry)
			if firstErr == nil {
				firstErr = pubErr
			}
		} else {
			replayed++
		}
	}

	b.dlqMu.Lock()
	b.dlq = remaining
	b.dlqMu.Unlock()

	return replayed, firstErr
}

func (b *Bus) PurgeDLQ() int {
	b.dlqMu.Lock()
	defer b.dlqMu.Unlock()
	n := len(b.dlq)
	b.dlq = nil
	return n
}

func (b *Bus) incMetric(topic, status string) {
	if b.metrics != nil {
		b.metrics.PublishTotal.WithLabelValues(topic, status).Inc()
	}
}
