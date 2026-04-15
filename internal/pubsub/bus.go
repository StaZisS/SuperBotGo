package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"SuperBotGo/internal/metrics"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const pgChannel = "admin_events"

type Bus struct {
	pool       *pgxpool.Pool
	connString string
	instanceID string
	metrics    *metrics.Metrics
}

func NewBus(pool *pgxpool.Pool, connString string, instanceID string) *Bus {
	return &Bus{pool: pool, connString: connString, instanceID: instanceID}
}

func (b *Bus) SetMetrics(metricSet *metrics.Metrics) {
	b.metrics = metricSet
}

func (b *Bus) InstanceID() string {
	return b.instanceID
}

func (b *Bus) Publish(ctx context.Context, event AdminEvent) error {
	event.InstanceID = b.instanceID
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = b.pool.Exec(ctx, "SELECT pg_notify($1, $2)", pgChannel, string(data))
	return err
}

func (b *Bus) Subscribe(ctx context.Context, handler func(AdminEvent)) error {
	for {
		if err := b.listenLoop(ctx, handler); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			b.incReconnect(reconnectReason(err))
			slog.Error("pubsub: listener disconnected, reconnecting in 3s", "error", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(3 * time.Second):
			}
		}
	}
}

func (b *Bus) incReconnect(reason string) {
	if b.metrics == nil {
		return
	}
	b.metrics.PubSubListenerReconnectTotal.WithLabelValues(reason).Inc()
}

func reconnectReason(err error) string {
	if err == nil {
		return "unknown"
	}
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "connect:"):
		return "connect"
	case strings.HasPrefix(msg, "listen:"):
		return "listen"
	case strings.HasPrefix(msg, "wait:"):
		return "wait"
	default:
		return "unknown"
	}
}

func (b *Bus) listenLoop(ctx context.Context, handler func(AdminEvent)) error {
	conn, err := pgx.Connect(ctx, b.connString)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, "LISTEN "+pgChannel); err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	slog.Info("pubsub: listening on PG channel", "channel", pgChannel)

	for {
		notification, err := conn.WaitForNotification(ctx)
		if err != nil {
			return fmt.Errorf("wait: %w", err)
		}

		var event AdminEvent
		if err := json.Unmarshal([]byte(notification.Payload), &event); err != nil {
			slog.Error("pubsub: failed to unmarshal event", "error", err, "payload", notification.Payload)
			continue
		}
		if event.InstanceID == b.instanceID {
			continue
		}
		handler(event)
	}
}
