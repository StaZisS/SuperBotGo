package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	appmetrics "SuperBotGo/internal/metrics"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresBus struct {
	pool       *pgxpool.Pool
	instanceID string
	cfg        Config
	metrics    *Metrics
	appm       *appmetrics.Metrics
}

type queuedEvent struct {
	ID             int64
	Topic          string
	Payload        []byte
	SourcePluginID string
	RetryCount     int
}

func NewPostgresBus(pool *pgxpool.Pool, instanceID string, cfg *Config, m *Metrics) *PostgresBus {
	var conf Config
	if cfg != nil {
		conf = cfg.withDefaults()
	} else {
		conf = (&Config{}).withDefaults()
	}
	return &PostgresBus{
		pool:       pool,
		instanceID: instanceID,
		cfg:        conf,
		metrics:    m,
	}
}

func (b *PostgresBus) SetAppMetrics(m *appmetrics.Metrics) {
	b.appm = m
}

func (b *PostgresBus) Publish(ctx context.Context, topic string, payload []byte) error {
	if !json.Valid(payload) {
		b.incMetric(topic, "invalid_payload")
		b.incAppMetric("postgres", "invalid_payload")
		return fmt.Errorf("event payload for topic %q is not valid JSON", topic)
	}

	source := pluginIDFromContext(ctx)
	_, err := b.pool.Exec(ctx, `
		INSERT INTO wasm_event_queue (topic, payload, source_plugin_id)
		VALUES ($1, $2::jsonb, $3)
	`, topic, string(payload), source)
	if err != nil {
		b.incMetric(topic, "publish_error")
		b.incAppMetric("postgres", "publish_error")
		return fmt.Errorf("enqueue event %q: %w", topic, err)
	}

	b.incMetric(topic, "enqueued")
	b.incAppMetric("postgres", "enqueued")
	return nil
}

func (b *PostgresBus) RunConsumer(ctx context.Context, handler Handler) error {
	for {
		processed, err := b.consumeOnce(ctx, handler)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Error("eventbus: postgres consumer iteration failed", "instance", b.instanceID, "error", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(b.cfg.PollInterval):
			}
			continue
		}
		if processed == 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(b.cfg.PollInterval):
			}
		}
	}
}

func (b *PostgresBus) consumeOnce(ctx context.Context, handler Handler) (int, error) {
	events, err := b.claimBatch(ctx)
	if err != nil {
		return 0, err
	}
	if len(events) == 0 {
		return 0, nil
	}

	var firstErr error
	for _, event := range events {
		eventCtx := ContextWithPluginID(ctx, event.SourcePluginID)
		if err := handler(eventCtx, event.Topic, event.Payload); err != nil {
			if markErr := b.markFailure(ctx, event, err); markErr != nil && firstErr == nil {
				firstErr = markErr
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := b.markSuccess(ctx, event.ID); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return len(events), firstErr
}

func (b *PostgresBus) claimBatch(ctx context.Context) ([]queuedEvent, error) {
	rows, err := b.pool.Query(ctx, `
		WITH next_events AS (
			SELECT id
			FROM wasm_event_queue
			WHERE
				(status = 'pending' AND available_at <= now())
				OR (status = 'processing' AND locked_until <= now())
			ORDER BY id
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE wasm_event_queue q
		SET
			status = 'processing',
			claimed_by = $2,
			locked_until = now() + ($3 * interval '1 millisecond'),
			updated_at = now()
		FROM next_events
		WHERE q.id = next_events.id
		RETURNING q.id, q.topic, q.payload::text, COALESCE(q.source_plugin_id, ''), q.retry_count
	`, b.cfg.BatchSize, b.instanceID, b.cfg.LeaseDuration.Milliseconds())
	if err != nil {
		return nil, fmt.Errorf("claim event batch: %w", err)
	}
	defer rows.Close()

	var events []queuedEvent
	for rows.Next() {
		var event queuedEvent
		var payload string
		if err := rows.Scan(&event.ID, &event.Topic, &payload, &event.SourcePluginID, &event.RetryCount); err != nil {
			return nil, fmt.Errorf("scan queued event: %w", err)
		}
		event.Payload = []byte(payload)
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate queued events: %w", err)
	}
	return events, nil
}

func (b *PostgresBus) markSuccess(ctx context.Context, id int64) error {
	if _, err := b.pool.Exec(ctx, `DELETE FROM wasm_event_queue WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete delivered event %d: %w", id, err)
	}
	b.incAppMetric("postgres", "delivered")
	return nil
}

func (b *PostgresBus) markFailure(ctx context.Context, event queuedEvent, handlerErr error) error {
	retries := event.RetryCount + 1
	if retries > b.cfg.MaxRetries {
		_, err := b.pool.Exec(ctx, `
			UPDATE wasm_event_queue
			SET
				status = 'dead',
				retry_count = $2,
				last_error = $3,
				dead_lettered_at = now(),
				locked_until = NULL,
				claimed_by = NULL,
				updated_at = now()
			WHERE id = $1
		`, event.ID, retries, handlerErr.Error())
		if err != nil {
			return fmt.Errorf("mark event %d dead-lettered: %w", event.ID, err)
		}
		b.incMetric(event.Topic, "dead_lettered")
		b.incAppMetric("postgres", "dead_lettered")
		return nil
	}

	delay := b.backoffDelay(event.RetryCount)
	_, err := b.pool.Exec(ctx, `
		UPDATE wasm_event_queue
		SET
			status = 'pending',
			retry_count = $2,
			last_error = $3,
			available_at = now() + ($4 * interval '1 millisecond'),
			locked_until = NULL,
			claimed_by = NULL,
			updated_at = now()
		WHERE id = $1
	`, event.ID, retries, handlerErr.Error(), delay.Milliseconds())
	if err != nil {
		return fmt.Errorf("requeue event %d: %w", event.ID, err)
	}
	b.incMetric(event.Topic, "retrying")
	b.incAppMetric("postgres", "retrying")
	return nil
}

func (b *PostgresBus) backoffDelay(retryIndex int) time.Duration {
	if retryIndex < len(b.cfg.Backoff) {
		return b.cfg.Backoff[retryIndex]
	}
	return b.cfg.Backoff[len(b.cfg.Backoff)-1]
}

func (b *PostgresBus) incMetric(topic, status string) {
	if b.metrics != nil {
		b.metrics.PublishTotal.WithLabelValues(topic, status).Inc()
	}
}

func (b *PostgresBus) incAppMetric(backend, status string) {
	if b.appm != nil {
		b.appm.PluginEventDeliveryTotal.WithLabelValues(backend, status).Inc()
	}
}
