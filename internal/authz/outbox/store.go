package outbox

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"

	"SuperBotGo/internal/authz/tuples"
)

const insertSQL = `INSERT INTO authz_outbox (operation, payload) VALUES ($1, $2)`
const notifySQL = `SELECT pg_notify('authz_outbox', '')`

func enqueue(ctx context.Context, tx pgx.Tx, op string, p Payload) error {
	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal outbox payload: %w", err)
	}
	if _, err := tx.Exec(ctx, insertSQL, op, data); err != nil {
		return fmt.Errorf("insert outbox %s: %w", op, err)
	}
	if _, err := tx.Exec(ctx, notifySQL); err != nil {
		return fmt.Errorf("notify outbox: %w", err)
	}
	return nil
}

// EnqueueTouch enqueues a TOUCH (create-or-update) operation.
func EnqueueTouch(ctx context.Context, tx pgx.Tx, tt []tuples.Tuple) error {
	if len(tt) == 0 {
		return nil
	}
	return enqueue(ctx, tx, OpTouch, Payload{Tuples: FromTuples(tt)})
}

// EnqueueDelete enqueues a DELETE of exact tuples.
func EnqueueDelete(ctx context.Context, tx pgx.Tx, tt []tuples.Tuple) error {
	if len(tt) == 0 {
		return nil
	}
	return enqueue(ctx, tx, OpDelete, Payload{Tuples: FromTuples(tt)})
}

// EnqueueDeleteByObject enqueues deletion of all tuples matching (objectType, objectID, relation).
func EnqueueDeleteByObject(ctx context.Context, tx pgx.Tx, objectType, objectID, relation string) error {
	return enqueue(ctx, tx, OpDeleteByObject, Payload{
		ObjectType: objectType,
		ObjectID:   objectID,
		Relation:   relation,
	})
}

// EnqueueDeleteBySubject enqueues deletion of all tuples matching (subjectType, subjectID, relation).
func EnqueueDeleteBySubject(ctx context.Context, tx pgx.Tx, subjectType, subjectID, relation string) error {
	return enqueue(ctx, tx, OpDeleteBySubject, Payload{
		SubjectType: subjectType,
		SubjectID:   subjectID,
		Relation:    relation,
	})
}

// EnqueueReplace enqueues an atomic replace: delete-by-object then write new tuples.
func EnqueueReplace(ctx context.Context, tx pgx.Tx, objectType, objectID, relation string, tt []tuples.Tuple) error {
	return enqueue(ctx, tx, OpReplace, Payload{
		ObjectType: objectType,
		ObjectID:   objectID,
		Relation:   relation,
		Tuples:     FromTuples(tt),
	})
}
