package hostapi

import (
	"context"
	"crypto/rand"
	"fmt"
)

type traceIDKey struct{}

func GenerateTraceID() string {
	var buf [16]byte
	_, _ = rand.Read(buf[:])
	return fmt.Sprintf("%x", buf)
}

func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	if existing := TraceIDFromContext(ctx); existing != "" {
		return ctx
	}
	return context.WithValue(ctx, traceIDKey{}, traceID)
}

func TraceIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(traceIDKey{}).(string); ok {
		return id
	}
	return ""
}
