package tuples

import v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"

// Tuple represents an authorization relationship.
type Tuple struct {
	ObjectType  string
	ObjectID    string
	Relation    string
	SubjectType string
	SubjectID   string
}

func (t Tuple) toRelationship() *v1.Relationship {
	return &v1.Relationship{
		Resource: &v1.ObjectReference{
			ObjectType: t.ObjectType,
			ObjectId:   t.ObjectID,
		},
		Relation: t.Relation,
		Subject: &v1.SubjectReference{
			Object: &v1.ObjectReference{
				ObjectType: t.SubjectType,
				ObjectId:   t.SubjectID,
			},
		},
	}
}
