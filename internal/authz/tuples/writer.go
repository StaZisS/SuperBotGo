package tuples

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// WriteTuples inserts multiple tuples within the given tx.
// Existing tuples are silently skipped (ON CONFLICT DO NOTHING).
func WriteTuples(ctx context.Context, tx pgx.Tx, tt []Tuple) error {
	for _, t := range tt {
		if _, err := tx.Exec(ctx, `
			INSERT INTO authorization_tuples (object_type, object_id, relation, subject_type, subject_id)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (object_type, object_id, relation, subject_type, subject_id) DO NOTHING
		`, t.ObjectType, t.ObjectID, t.Relation, t.SubjectType, t.SubjectID); err != nil {
			return fmt.Errorf("write tuple (%s:%s#%s@%s:%s): %w",
				t.ObjectType, t.ObjectID, t.Relation, t.SubjectType, t.SubjectID, err)
		}
	}
	return nil
}

// DeleteTuples removes exact tuples within tx.
func DeleteTuples(ctx context.Context, tx pgx.Tx, tt []Tuple) error {
	for _, t := range tt {
		if _, err := tx.Exec(ctx, `
			DELETE FROM authorization_tuples
			WHERE object_type = $1 AND object_id = $2 AND relation = $3
			  AND subject_type = $4 AND subject_id = $5
		`, t.ObjectType, t.ObjectID, t.Relation, t.SubjectType, t.SubjectID); err != nil {
			return fmt.Errorf("delete tuple (%s:%s#%s@%s:%s): %w",
				t.ObjectType, t.ObjectID, t.Relation, t.SubjectType, t.SubjectID, err)
		}
	}
	return nil
}

// DeleteByObject removes all tuples for a given object+relation within tx.
func DeleteByObject(ctx context.Context, tx pgx.Tx, objectType, objectID, relation string) error {
	_, err := tx.Exec(ctx, `
		DELETE FROM authorization_tuples
		WHERE object_type = $1 AND object_id = $2 AND relation = $3
	`, objectType, objectID, relation)
	return err
}

// DeleteBySubject removes all tuples for a given subject+relation within tx.
func DeleteBySubject(ctx context.Context, tx pgx.Tx, subjectType, subjectID, relation string) error {
	_, err := tx.Exec(ctx, `
		DELETE FROM authorization_tuples
		WHERE subject_type = $1 AND subject_id = $2 AND relation = $3
	`, subjectType, subjectID, relation)
	return err
}

// ReplaceForObject atomically replaces all tuples for a given object+relation.
func ReplaceForObject(ctx context.Context, tx pgx.Tx, objectType, objectID, relation string, newTuples []Tuple) error {
	if err := DeleteByObject(ctx, tx, objectType, objectID, relation); err != nil {
		return err
	}
	return WriteTuples(ctx, tx, newTuples)
}
