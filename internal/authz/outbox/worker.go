package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"SuperBotGo/internal/authz/tuples"
)

const (
	defaultBatchSize    = 50
	defaultPollInterval = 5 * time.Second
	maxAttempts         = 10
)

// Worker processes outbox entries and dispatches them to SpiceDB.
type Worker struct {
	pool         *pgxpool.Pool
	writer       *tuples.Writer
	logger       *slog.Logger
	batchSize    int
	pollInterval time.Duration
}

func NewWorker(pool *pgxpool.Pool, writer *tuples.Writer, logger *slog.Logger) *Worker {
	return &Worker{
		pool:         pool,
		writer:       writer,
		logger:       logger,
		batchSize:    defaultBatchSize,
		pollInterval: defaultPollInterval,
	}
}

// Run starts the processing loop. Blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	// Acquire a dedicated connection for LISTEN.
	conn, err := w.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire listen conn: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN authz_outbox"); err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	w.logger.Info("authz outbox worker started")

	// Process any pending entries on startup.
	w.processBatch(ctx)

	for {
		// Wait for notification or poll timeout.
		_, err := conn.Conn().WaitForNotification(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			w.logger.Warn("outbox listen error, falling back to poll", slog.Any("error", err))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(w.pollInterval):
			}
		}

		w.processBatch(ctx)
	}
}

type outboxRow struct {
	ID        int64
	Operation string
	Payload   []byte
	Attempts  int
}

func (w *Worker) processBatch(ctx context.Context) {
	for {
		n, err := w.processOnce(ctx)
		if err != nil {
			w.logger.Error("outbox batch error", slog.Any("error", err))
			return
		}
		if n == 0 {
			return
		}
	}
}

func (w *Worker) processOnce(ctx context.Context) (int, error) {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT id, operation, payload, attempts
		FROM authz_outbox
		WHERE processed_at IS NULL
		  AND (locked_until IS NULL OR locked_until < now())
		  AND attempts < $1
		ORDER BY id
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`, maxAttempts, w.batchSize)
	if err != nil {
		return 0, fmt.Errorf("select outbox: %w", err)
	}

	var batch []outboxRow
	for rows.Next() {
		var r outboxRow
		if err := rows.Scan(&r.ID, &r.Operation, &r.Payload, &r.Attempts); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan outbox row: %w", err)
		}
		batch = append(batch, r)
	}
	rows.Close()
	if rows.Err() != nil {
		return 0, rows.Err()
	}

	if len(batch) == 0 {
		return 0, tx.Commit(ctx)
	}

	for _, r := range batch {
		if err := w.dispatch(ctx, r); err != nil {
			w.logger.Warn("outbox dispatch failed",
				slog.Int64("id", r.ID),
				slog.String("op", r.Operation),
				slog.Any("error", err),
			)
			backoff := time.Duration(math.Pow(2, float64(r.Attempts+1))) * time.Second
			if backoff > 5*time.Minute {
				backoff = 5 * time.Minute
			}
			if _, err := tx.Exec(ctx, `
				UPDATE authz_outbox
				SET attempts = attempts + 1, last_error = $2, locked_until = now() + $3::interval
				WHERE id = $1
			`, r.ID, err.Error(), fmt.Sprintf("%d seconds", int(backoff.Seconds()))); err != nil {
				return 0, fmt.Errorf("update failed row %d: %w", r.ID, err)
			}
			continue
		}

		if _, err := tx.Exec(ctx, `
			UPDATE authz_outbox SET processed_at = now() WHERE id = $1
		`, r.ID); err != nil {
			return 0, fmt.Errorf("mark processed %d: %w", r.ID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return len(batch), nil
}

func (w *Worker) dispatch(ctx context.Context, r outboxRow) error {
	var p Payload
	if err := json.Unmarshal(r.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	switch r.Operation {
	case OpTouch:
		return w.writer.WriteTuples(ctx, ToTuples(p.Tuples))
	case OpDelete:
		return w.writer.DeleteTuples(ctx, ToTuples(p.Tuples))
	case OpDeleteByObject:
		return w.writer.DeleteByObject(ctx, p.ObjectType, p.ObjectID, p.Relation)
	case OpDeleteBySubject:
		return w.writer.DeleteBySubject(ctx, p.SubjectType, p.SubjectID, p.Relation)
	case OpReplace:
		return w.writer.ReplaceForObject(ctx, p.ObjectType, p.ObjectID, p.Relation, ToTuples(p.Tuples))
	default:
		return fmt.Errorf("unknown operation: %s", r.Operation)
	}
}

// Cleanup removes processed entries older than the given duration.
func Cleanup(ctx context.Context, pool *pgxpool.Pool, olderThan time.Duration) (int64, error) {
	tag, err := pool.Exec(ctx, `
		DELETE FROM authz_outbox
		WHERE processed_at IS NOT NULL AND processed_at < now() - $1::interval
	`, fmt.Sprintf("%d seconds", int(olderThan.Seconds())))
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// PendingCount returns the number of unprocessed outbox entries.
func PendingCount(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	var count int64
	err := pool.QueryRow(ctx, `SELECT count(*) FROM authz_outbox WHERE processed_at IS NULL`).Scan(&count)
	return count, err
}
