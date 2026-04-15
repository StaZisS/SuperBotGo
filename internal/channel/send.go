package channel

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net"
	"strings"
	"time"

	"SuperBotGo/internal/model"
)

const (
	maxRetries    = 3
	baseDelay     = 500 * time.Millisecond
	maxDelay      = 5 * time.Second
	jitterPercent = 0.3
)

// SilentSender is an optional interface that adapters implement to support
// silent (notification-suppressed) message delivery.
type SilentSender interface {
	SendToUserSilent(ctx context.Context, platformUserID model.PlatformUserID, msg model.Message, silent bool) error
	SendToChatSilent(ctx context.Context, chatID string, msg model.Message, silent bool) error
}

func withRetry(ctx context.Context, fn func() error, onRetry func()) error {
	var lastErr error
	for attempt := range maxRetries {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if !isTransient(lastErr) {
			return lastErr
		}
		if attempt == maxRetries-1 {
			break
		}

		delay := backoffDelay(attempt)
		if onRetry != nil {
			onRetry()
		}
		slog.Warn("send failed, retrying",
			slog.Int("attempt", attempt+1),
			slog.Duration("delay", delay),
			slog.Any("error", lastErr))

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return lastErr
}

func backoffDelay(attempt int) time.Duration {
	delay := baseDelay << uint(attempt) // exponential: 500ms, 1s, 2s
	if delay > maxDelay {
		delay = maxDelay
	}
	jitter := time.Duration(float64(delay) * jitterPercent * (rand.Float64()*2 - 1))
	return delay + jitter
}

var transientPatterns = []string{
	"429",
	"too many requests",
	"retry after",
	"connection reset",
	"connection refused",
	"eof",
	"timeout",
	"temporary failure",
	"service unavailable",
	"bad gateway",
	"gateway timeout",
	"internal server error",
}

func isTransient(err error) bool {
	if err == nil {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	msg := strings.ToLower(err.Error())
	for _, pattern := range transientPatterns {
		if strings.Contains(msg, pattern) {
			return true
		}
	}

	return false
}
