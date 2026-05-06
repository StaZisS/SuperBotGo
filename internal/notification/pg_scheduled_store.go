package notification

import (
	"context"
	"fmt"
	"time"

	"SuperBotGo/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PgScheduledStore persists delayed notifications in PostgreSQL.
type PgScheduledStore struct {
	pool *pgxpool.Pool
}

func NewPgScheduledStore(pool *pgxpool.Pool) *PgScheduledStore {
	return &PgScheduledStore{pool: pool}
}

func (s *PgScheduledStore) EnsureSchema(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS scheduled_notifications (
			id             BIGSERIAL    PRIMARY KEY,
			global_user_id BIGINT       NOT NULL,
			message        JSONB        NOT NULL,
			priority       SMALLINT     NOT NULL,
			send_at        TIMESTAMPTZ  NOT NULL,
			created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
			attempts       INTEGER      NOT NULL DEFAULT 0,
			locked_until   TIMESTAMPTZ,
			last_error     TEXT,
			updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
		);

		ALTER TABLE scheduled_notifications ADD COLUMN IF NOT EXISTS id BIGSERIAL;
		ALTER TABLE scheduled_notifications ADD COLUMN IF NOT EXISTS global_user_id BIGINT;
		ALTER TABLE scheduled_notifications ADD COLUMN IF NOT EXISTS message JSONB;
		ALTER TABLE scheduled_notifications ADD COLUMN IF NOT EXISTS priority SMALLINT DEFAULT 0;
		ALTER TABLE scheduled_notifications ADD COLUMN IF NOT EXISTS send_at TIMESTAMPTZ DEFAULT now();
		ALTER TABLE scheduled_notifications ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ DEFAULT now();
		ALTER TABLE scheduled_notifications ADD COLUMN IF NOT EXISTS attempts INTEGER DEFAULT 0;
		ALTER TABLE scheduled_notifications ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ;
		ALTER TABLE scheduled_notifications ADD COLUMN IF NOT EXISTS last_error TEXT;
		ALTER TABLE scheduled_notifications ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT now();

		UPDATE scheduled_notifications
		SET send_at = COALESCE(send_at, now()),
		    created_at = COALESCE(created_at, now()),
		    attempts = COALESCE(attempts, 0),
		    updated_at = COALESCE(updated_at, now());

		ALTER TABLE scheduled_notifications ALTER COLUMN send_at SET NOT NULL;
		ALTER TABLE scheduled_notifications ALTER COLUMN created_at SET NOT NULL;
		ALTER TABLE scheduled_notifications ALTER COLUMN attempts SET NOT NULL;
		ALTER TABLE scheduled_notifications ALTER COLUMN updated_at SET NOT NULL;

		CREATE INDEX IF NOT EXISTS idx_scheduled_notifications_due
			ON scheduled_notifications (send_at, id)
			WHERE locked_until IS NULL;

		CREATE INDEX IF NOT EXISTS idx_scheduled_notifications_locked_until
			ON scheduled_notifications (locked_until)
			WHERE locked_until IS NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("notification: ensure scheduled_notifications schema: %w", err)
	}
	return nil
}

func (s *PgScheduledStore) Enqueue(ctx context.Context, msg ScheduledMessage) error {
	payload, err := marshalMessage(msg.Msg)
	if err != nil {
		return err
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO scheduled_notifications (global_user_id, message, priority, send_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, msg.UserID, payload, int(msg.Priority), msg.SendAt, msg.CreatedAt)
	if err != nil {
		return fmt.Errorf("notification: enqueue scheduled message for user %d: %w", msg.UserID, err)
	}
	return nil
}

func (s *PgScheduledStore) ClaimDue(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]ScheduledMessage, error) {
	if limit <= 0 {
		return nil, nil
	}

	rows, err := s.pool.Query(ctx, `
		WITH due AS (
			SELECT id
			FROM scheduled_notifications
			WHERE send_at <= $1
			  AND (locked_until IS NULL OR locked_until <= $1)
			ORDER BY send_at, id
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		UPDATE scheduled_notifications sn
		SET locked_until = $3,
		    attempts = attempts + 1,
		    updated_at = now()
		FROM due
		WHERE sn.id = due.id
		RETURNING sn.id, sn.global_user_id, sn.message, sn.priority, sn.send_at, sn.created_at, sn.attempts
	`, now, limit, now.Add(lease))
	if err != nil {
		return nil, fmt.Errorf("notification: claim scheduled messages: %w", err)
	}
	defer rows.Close()

	var messages []ScheduledMessage
	for rows.Next() {
		var (
			msg      ScheduledMessage
			payload  []byte
			priority int
		)
		if err := rows.Scan(&msg.ID, &msg.UserID, &payload, &priority, &msg.SendAt, &msg.CreatedAt, &msg.Attempts); err != nil {
			return nil, fmt.Errorf("notification: scan scheduled message: %w", err)
		}
		msg.Msg, err = unmarshalMessage(payload)
		if err != nil {
			return nil, fmt.Errorf("notification: decode scheduled message %d: %w", msg.ID, err)
		}
		msg.Priority = toNotifyPriority(priority)
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("notification: iterate scheduled messages: %w", err)
	}
	return messages, nil
}

func (s *PgScheduledStore) Complete(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM scheduled_notifications WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("notification: complete scheduled message %d: %w", id, err)
	}
	return nil
}

func (s *PgScheduledStore) Reschedule(ctx context.Context, id int64, sendAt time.Time, reason error) error {
	var lastError *string
	if reason != nil {
		errText := reason.Error()
		lastError = &errText
	}

	_, err := s.pool.Exec(ctx, `
		UPDATE scheduled_notifications
		SET send_at = $2,
		    locked_until = NULL,
		    last_error = $3,
		    updated_at = now()
		WHERE id = $1
	`, id, sendAt, lastError)
	if err != nil {
		return fmt.Errorf("notification: reschedule scheduled message %d: %w", id, err)
	}
	return nil
}

func toNotifyPriority(priority int) model.NotifyPriority {
	if priority < int(model.PriorityLow) {
		return model.PriorityLow
	}
	if priority > int(model.PriorityCritical) {
		return model.PriorityCritical
	}
	return model.NotifyPriority(priority)
}

var _ ScheduledMessageStore = (*PgScheduledStore)(nil)
