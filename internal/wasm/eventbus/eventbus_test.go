package eventbus

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestPublish_NoHandlers(t *testing.T) {
	bus := New(nil, nil)
	if err := bus.Publish(context.Background(), "test.topic", []byte(`{}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPublish_SingleHandler_Success(t *testing.T) {
	bus := New(nil, nil)

	var received int
	bus.Subscribe(func(_ context.Context, topic string, payload []byte) error {
		received++
		return nil
	})

	if err := bus.Publish(context.Background(), "test.topic", []byte(`{"key":"value"}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if received != 1 {
		t.Fatalf("expected handler to be called once, got %d", received)
	}
}

func TestPublish_RetryOnFailure(t *testing.T) {
	bus := New(&Config{
		MaxRetries: 3,
		Backoff:    []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond},
	}, nil)

	var calls int
	bus.Subscribe(func(_ context.Context, topic string, payload []byte) error {
		calls++
		if calls < 3 {
			return errors.New("transient error")
		}
		return nil
	})

	if err := bus.Publish(context.Background(), "test.topic", []byte(`{}`)); err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls (1 initial + 2 retries), got %d", calls)
	}
	if bus.DLQLen() != 0 {
		t.Fatalf("expected empty DLQ, got %d entries", bus.DLQLen())
	}
}

func TestPublish_ExhaustedRetries_AddsToDLQ(t *testing.T) {
	bus := New(&Config{
		MaxRetries: 2,
		Backoff:    []time.Duration{time.Millisecond, time.Millisecond},
	}, nil)

	bus.Subscribe(func(_ context.Context, topic string, payload []byte) error {
		return errors.New("permanent error")
	})

	ctx := ContextWithPluginID(context.Background(), "my-plugin")
	err := bus.Publish(ctx, "test.topic", []byte(`{"data":1}`))
	if err == nil {
		t.Fatal("expected error when all retries exhausted")
	}

	entries := bus.DLQEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 DLQ entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Topic != "test.topic" {
		t.Errorf("DLQ topic = %q, want %q", e.Topic, "test.topic")
	}
	if e.PluginID != "my-plugin" {
		t.Errorf("DLQ plugin_id = %q, want %q", e.PluginID, "my-plugin")
	}
	if e.Error != "permanent error" {
		t.Errorf("DLQ error = %q, want %q", e.Error, "permanent error")
	}
	if e.RetryCount != 2 {
		t.Errorf("DLQ retry_count = %d, want %d", e.RetryCount, 2)
	}
}

func TestPublish_MultipleHandlers(t *testing.T) {
	bus := New(&Config{
		MaxRetries: 1,
		Backoff:    []time.Duration{time.Millisecond},
	}, nil)

	var ok1, ok2 atomic.Int32
	bus.Subscribe(func(_ context.Context, _ string, _ []byte) error {
		ok1.Add(1)
		return nil
	})
	bus.Subscribe(func(_ context.Context, _ string, _ []byte) error {
		ok2.Add(1)
		return errors.New("handler 2 fails")
	})

	err := bus.Publish(context.Background(), "multi", []byte(`{}`))
	if err == nil {
		t.Fatal("expected error from failing handler")
	}
	if ok1.Load() != 1 {
		t.Errorf("handler 1 calls = %d, want 1", ok1.Load())
	}
	// handler 2: 1 initial + 1 retry = 2 calls
	if ok2.Load() != 2 {
		t.Errorf("handler 2 calls = %d, want 2", ok2.Load())
	}
	if bus.DLQLen() != 1 {
		t.Errorf("DLQ len = %d, want 1", bus.DLQLen())
	}
}

func TestDLQ_MaxSize_Eviction(t *testing.T) {
	bus := New(&Config{
		MaxRetries: 1,
		Backoff:    []time.Duration{time.Millisecond},
		DLQMaxSize: 3,
	}, nil)

	bus.Subscribe(func(_ context.Context, _ string, _ []byte) error {
		return errors.New("fail")
	})

	for i := 0; i < 5; i++ {
		_ = bus.Publish(context.Background(), "evict", []byte(`{}`))
	}

	if bus.DLQLen() != 3 {
		t.Fatalf("DLQ len = %d, want 3 (max size)", bus.DLQLen())
	}

	entries := bus.DLQEntries()
	// Oldest 2 should have been evicted; remaining IDs are 3, 4, 5.
	if entries[0].ID != "dlq-3" {
		t.Errorf("oldest DLQ entry ID = %q, want %q", entries[0].ID, "dlq-3")
	}
}

func TestReplayDLQ_Success(t *testing.T) {
	bus := New(&Config{
		MaxRetries: 1,
		Backoff:    []time.Duration{time.Millisecond},
	}, nil)

	var calls atomic.Int32
	bus.Subscribe(func(_ context.Context, _ string, _ []byte) error {
		n := calls.Add(1)
		// Fail on first 2 calls (initial + 1 retry), succeed on replay.
		if n <= 2 {
			return errors.New("fail")
		}
		return nil
	})

	ctx := ContextWithPluginID(context.Background(), "replay-plugin")
	_ = bus.Publish(ctx, "replay.topic", []byte(`{}`))
	if bus.DLQLen() != 1 {
		t.Fatalf("DLQ len = %d, want 1 after failed publish", bus.DLQLen())
	}

	replayed, err := bus.ReplayDLQ(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if replayed != 1 {
		t.Fatalf("replayed = %d, want 1", replayed)
	}
	if bus.DLQLen() != 0 {
		t.Fatalf("DLQ len = %d, want 0 after replay", bus.DLQLen())
	}
}

func TestReplayDLQ_StillFailing(t *testing.T) {
	bus := New(&Config{
		MaxRetries: 1,
		Backoff:    []time.Duration{time.Millisecond},
	}, nil)

	bus.Subscribe(func(_ context.Context, _ string, _ []byte) error {
		return errors.New("always fails")
	})

	ctx := ContextWithPluginID(context.Background(), "p1")
	_ = bus.Publish(ctx, "bad.topic", []byte(`{}`))

	replayed, err := bus.ReplayDLQ(context.Background())
	if err == nil {
		t.Fatal("expected error when replay still fails")
	}
	if replayed != 0 {
		t.Fatalf("replayed = %d, want 0", replayed)
	}
	if bus.DLQLen() != 1 {
		t.Fatalf("DLQ len = %d, want 1 (entry should remain)", bus.DLQLen())
	}
}

func TestPurgeDLQ(t *testing.T) {
	bus := New(&Config{
		MaxRetries: 1,
		Backoff:    []time.Duration{time.Millisecond},
	}, nil)

	bus.Subscribe(func(_ context.Context, _ string, _ []byte) error {
		return errors.New("fail")
	})

	_ = bus.Publish(context.Background(), "a", []byte(`{}`))
	_ = bus.Publish(context.Background(), "b", []byte(`{}`))

	purged := bus.PurgeDLQ()
	if purged != 2 {
		t.Fatalf("purged = %d, want 2", purged)
	}
	if bus.DLQLen() != 0 {
		t.Fatalf("DLQ len = %d, want 0 after purge", bus.DLQLen())
	}
}

func TestPublish_ContextCancelled_AbortsRetry(t *testing.T) {
	bus := New(&Config{
		MaxRetries: 3,
		Backoff:    []time.Duration{50 * time.Millisecond, 50 * time.Millisecond, 50 * time.Millisecond},
	}, nil)

	var calls atomic.Int32
	bus.Subscribe(func(_ context.Context, _ string, _ []byte) error {
		calls.Add(1)
		return errors.New("fail")
	})

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after 30ms — should abort during first backoff wait.
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	err := bus.Publish(ctx, "cancel", []byte(`{}`))
	if err == nil {
		t.Fatal("expected error")
	}
	// Should have done initial call but been cut short during backoff.
	if calls.Load() > 2 {
		t.Errorf("expected at most 2 calls (context should cancel), got %d", calls.Load())
	}
}

func TestContextWithPluginID(t *testing.T) {
	ctx := ContextWithPluginID(context.Background(), "test-id")
	if got := pluginIDFromContext(ctx); got != "test-id" {
		t.Fatalf("pluginIDFromContext = %q, want %q", got, "test-id")
	}
}

func TestPluginIDFromContext_Empty(t *testing.T) {
	if got := pluginIDFromContext(context.Background()); got != "" {
		t.Fatalf("pluginIDFromContext on empty context = %q, want %q", got, "")
	}
}

func TestBackoffDelay_Extrapolation(t *testing.T) {
	bus := New(&Config{
		MaxRetries: 5,
		Backoff:    []time.Duration{10 * time.Millisecond, 20 * time.Millisecond},
	}, nil)

	if d := bus.backoffDelay(0); d != 10*time.Millisecond {
		t.Errorf("backoff[0] = %v, want 10ms", d)
	}
	if d := bus.backoffDelay(1); d != 20*time.Millisecond {
		t.Errorf("backoff[1] = %v, want 20ms", d)
	}
	// Beyond the slice: should reuse last element.
	if d := bus.backoffDelay(2); d != 20*time.Millisecond {
		t.Errorf("backoff[2] = %v, want 20ms (reuse last)", d)
	}
	if d := bus.backoffDelay(10); d != 20*time.Millisecond {
		t.Errorf("backoff[10] = %v, want 20ms (reuse last)", d)
	}
}
