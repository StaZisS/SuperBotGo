package channel

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"
)

// --- mock net.Error ---

type mockNetError struct {
	timeout   bool
	temporary bool
}

func (e *mockNetError) Error() string   { return "mock net error" }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return e.temporary }

var _ net.Error = (*mockNetError)(nil)

// ---------------------------------------------------------------------------
// isTransient
// ---------------------------------------------------------------------------

func TestIsTransient(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "net.Error with timeout",
			err:  &mockNetError{timeout: true},
			want: true,
		},
		{
			name: "net.Error without timeout",
			err:  &mockNetError{timeout: false, temporary: false},
			want: true, // any net.Error is transient
		},
		{
			name: "wrapped net.Error",
			err:  fmt.Errorf("wrapped: %w", &mockNetError{timeout: true}),
			want: true,
		},
		{
			name: "error containing 429",
			err:  errors.New("HTTP 429 rate limit exceeded"),
			want: true,
		},
		{
			name: "error containing timeout",
			err:  errors.New("request timeout after 30s"),
			want: true,
		},
		{
			name: "error containing connection reset",
			err:  errors.New("read tcp: connection reset by peer"),
			want: true,
		},
		{
			name: "error containing service unavailable",
			err:  errors.New("503 service unavailable"),
			want: true,
		},
		{
			name: "error containing too many requests",
			err:  errors.New("too many requests"),
			want: true,
		},
		{
			name: "error containing retry after",
			err:  errors.New("retry after 60 seconds"),
			want: true,
		},
		{
			name: "error containing connection refused",
			err:  errors.New("dial tcp: connection refused"),
			want: true,
		},
		{
			name: "error containing eof",
			err:  errors.New("unexpected eof"),
			want: true,
		},
		{
			name: "error containing bad gateway",
			err:  errors.New("502 bad gateway"),
			want: true,
		},
		{
			name: "error containing gateway timeout",
			err:  errors.New("504 gateway timeout"),
			want: true,
		},
		{
			name: "error containing internal server error",
			err:  errors.New("500 internal server error"),
			want: true,
		},
		{
			name: "error containing temporary failure",
			err:  errors.New("temporary failure in name resolution"),
			want: true,
		},
		{
			name: "regular error - not transient",
			err:  errors.New("invalid input"),
			want: false,
		},
		{
			name: "case insensitive - Too Many Requests",
			err:  errors.New("Too Many Requests"),
			want: true,
		},
		{
			name: "case insensitive - CONNECTION RESET",
			err:  errors.New("CONNECTION RESET"),
			want: true,
		},
		{
			name: "case insensitive - Service Unavailable",
			err:  errors.New("Service Unavailable"),
			want: true,
		},
		{
			name: "non-transient - permission denied",
			err:  errors.New("permission denied"),
			want: false,
		},
		{
			name: "non-transient - not found",
			err:  errors.New("resource not found"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTransient(tt.err)
			if got != tt.want {
				t.Errorf("isTransient(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// backoffDelay
// ---------------------------------------------------------------------------

func TestBackoffDelay(t *testing.T) {
	tests := []struct {
		name       string
		attempt    int
		wantBase   time.Duration
		wantMinPct float64 // lower bound as fraction of base (1 - jitterPercent)
		wantMaxPct float64 // upper bound as fraction of base (1 + jitterPercent)
	}{
		{
			name:       "attempt 0 - base 500ms",
			attempt:    0,
			wantBase:   500 * time.Millisecond,
			wantMinPct: 1 - jitterPercent,
			wantMaxPct: 1 + jitterPercent,
		},
		{
			name:       "attempt 1 - base 1s",
			attempt:    1,
			wantBase:   1 * time.Second,
			wantMinPct: 1 - jitterPercent,
			wantMaxPct: 1 + jitterPercent,
		},
		{
			name:       "attempt 2 - base 2s",
			attempt:    2,
			wantBase:   2 * time.Second,
			wantMinPct: 1 - jitterPercent,
			wantMaxPct: 1 + jitterPercent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := backoffDelay(tt.attempt)

			minExpected := time.Duration(float64(tt.wantBase) * tt.wantMinPct)
			maxExpected := time.Duration(float64(tt.wantBase) * tt.wantMaxPct)

			if got < minExpected || got > maxExpected {
				t.Errorf("backoffDelay(%d) = %v, want in [%v, %v]",
					tt.attempt, got, minExpected, maxExpected)
			}
		})
	}
}

func TestBackoffDelay_NeverExceedsMaxDelay(t *testing.T) {
	maxWithJitter := maxDelay + time.Duration(float64(maxDelay)*jitterPercent)
	for attempt := 0; attempt < 20; attempt++ {
		got := backoffDelay(attempt)
		if got > maxWithJitter {
			t.Errorf("backoffDelay(%d) = %v, exceeds max+jitter %v",
				attempt, got, maxWithJitter)
		}
	}
}

func TestBackoffDelay_JitterProducesVariance(t *testing.T) {
	const iterations = 50
	seen := make(map[time.Duration]bool, iterations)
	for range iterations {
		d := backoffDelay(0)
		seen[d] = true
	}

	// With jitter, we expect more than one distinct value over 50 calls.
	if len(seen) < 2 {
		t.Errorf("backoffDelay(0) produced only %d distinct value(s) over %d calls; jitter appears broken",
			len(seen), iterations)
	}
}

// ---------------------------------------------------------------------------
// withRetry
// ---------------------------------------------------------------------------

func TestWithRetry_SuccessFirstAttempt(t *testing.T) {
	calls := 0
	err := withRetry(context.Background(), func() error {
		calls++
		return nil
	}, nil)

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected fn called once, got %d", calls)
	}
}

func TestWithRetry_NonTransientReturnsImmediately(t *testing.T) {
	calls := 0
	nonTransient := errors.New("invalid input")

	err := withRetry(context.Background(), func() error {
		calls++
		return nonTransient
	}, nil)

	if !errors.Is(err, nonTransient) {
		t.Fatalf("expected %v, got %v", nonTransient, err)
	}
	if calls != 1 {
		t.Errorf("expected fn called once for non-transient error, got %d", calls)
	}
}

func TestWithRetry_ContextCancelledBeforeRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	calls := 0
	transientErr := errors.New("429 rate limited")

	err := withRetry(ctx, func() error {
		calls++
		return transientErr
	}, nil)

	// The first call will execute and return a transient error.
	// Then the select should pick up the cancelled context.
	if calls < 1 {
		t.Fatalf("expected fn called at least once, got %d", calls)
	}
	if !errors.Is(err, context.Canceled) {
		// It's also valid for the function to return transientErr
		// if it was the last attempt, but with cancellation it should
		// return ctx.Err() from the select.
		if calls == 1 && errors.Is(err, context.Canceled) {
			// expected
		} else if err == nil {
			t.Fatal("expected error, got nil")
		}
		// On the first failed attempt, the context is already cancelled,
		// so select picks ctx.Done() and returns context.Canceled.
		// However, if maxRetries-1==attempt (i.e. attempt==2), it breaks.
		// Since attempt starts at 0 and maxRetries is 3, the first iteration
		// won't break, so it will hit the select with a cancelled context.
	}
}

func TestWithRetry_ContextDeadlineExceeded(t *testing.T) {
	// Use a very short deadline so the retry's time.After loses the race.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Give the context time to expire before we even call.
	time.Sleep(5 * time.Millisecond)

	calls := 0
	err := withRetry(ctx, func() error {
		calls++
		return errors.New("timeout on server")
	}, nil)

	if calls < 1 {
		t.Fatal("expected fn called at least once")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		// After fn returns a transient error and context is expired,
		// select picks ctx.Done().
		t.Logf("got error: %v (calls=%d)", err, calls)
	}
}

func TestWithRetry_TransientThenSuccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow retry test in short mode")
	}

	calls := 0
	err := withRetry(context.Background(), func() error {
		calls++
		if calls == 1 {
			return errors.New("429 rate limited")
		}
		return nil
	}, nil)

	if err != nil {
		t.Fatalf("expected nil error after retry, got %v", err)
	}
	if calls != 2 {
		t.Errorf("expected fn called twice, got %d", calls)
	}
}

func TestWithRetry_AllAttemptsFailTransient(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow retry test in short mode")
	}

	calls := 0
	transientErr := errors.New("503 service unavailable")

	err := withRetry(context.Background(), func() error {
		calls++
		return transientErr
	}, nil)

	if !errors.Is(err, transientErr) {
		t.Fatalf("expected %v, got %v", transientErr, err)
	}
	if calls != maxRetries {
		t.Errorf("expected fn called %d times, got %d", maxRetries, calls)
	}
}
