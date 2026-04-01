package tuples

import (
	"context"
	"fmt"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	authzed "github.com/authzed/authzed-go/v1"
)

// Writer manages SpiceDB relationships.
type Writer struct {
	client *authzed.Client
}

// NewWriter creates a new SpiceDB relationship writer.
func NewWriter(client *authzed.Client) *Writer {
	return &Writer{client: client}
}

// WriteTuples creates or touches relationships in SpiceDB.
func (w *Writer) WriteTuples(ctx context.Context, tt []Tuple) error {
	if len(tt) == 0 {
		return nil
	}
	updates := make([]*v1.RelationshipUpdate, len(tt))
	for i, t := range tt {
		updates[i] = &v1.RelationshipUpdate{
			Operation:    v1.RelationshipUpdate_OPERATION_TOUCH,
			Relationship: t.toRelationship(),
		}
	}
	_, err := w.client.WriteRelationships(ctx, &v1.WriteRelationshipsRequest{Updates: updates})
	if err != nil {
		return fmt.Errorf("spicedb write relationships: %w", err)
	}
	return nil
}

// DeleteTuples removes exact relationships from SpiceDB.
func (w *Writer) DeleteTuples(ctx context.Context, tt []Tuple) error {
	if len(tt) == 0 {
		return nil
	}
	updates := make([]*v1.RelationshipUpdate, len(tt))
	for i, t := range tt {
		updates[i] = &v1.RelationshipUpdate{
			Operation:    v1.RelationshipUpdate_OPERATION_DELETE,
			Relationship: t.toRelationship(),
		}
	}
	_, err := w.client.WriteRelationships(ctx, &v1.WriteRelationshipsRequest{Updates: updates})
	if err != nil {
		return fmt.Errorf("spicedb delete relationships: %w", err)
	}
	return nil
}

// DeleteByObject removes all relationships for an object/relation pair using DeleteRelationships API.
func (w *Writer) DeleteByObject(ctx context.Context, objectType, objectID, relation string) error {
	_, err := w.client.DeleteRelationships(ctx, &v1.DeleteRelationshipsRequest{
		RelationshipFilter: &v1.RelationshipFilter{
			ResourceType:       objectType,
			OptionalResourceId: objectID,
			OptionalRelation:   relation,
		},
	})
	if err != nil {
		return fmt.Errorf("spicedb delete by object %s:%s#%s: %w", objectType, objectID, relation, err)
	}
	return nil
}

// DeleteBySubject removes all relationships for a subject/relation pair.
func (w *Writer) DeleteBySubject(ctx context.Context, subjectType, subjectID, relation string) error {
	_, err := w.client.DeleteRelationships(ctx, &v1.DeleteRelationshipsRequest{
		RelationshipFilter: &v1.RelationshipFilter{
			ResourceType:     "",
			OptionalRelation: relation,
			OptionalSubjectFilter: &v1.SubjectFilter{
				SubjectType:       subjectType,
				OptionalSubjectId: subjectID,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("spicedb delete by subject %s:%s#%s: %w", subjectType, subjectID, relation, err)
	}
	return nil
}

// ReplaceForObject atomically replaces all relationships for an object/relation.
func (w *Writer) ReplaceForObject(ctx context.Context, objectType, objectID, relation string, newTuples []Tuple) error {
	if err := w.DeleteByObject(ctx, objectType, objectID, relation); err != nil {
		return err
	}
	return w.WriteTuples(ctx, newTuples)
}
