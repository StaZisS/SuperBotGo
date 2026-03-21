package tuples

// Tuple represents a single authorization relationship:
//
//	object_type:object_id#relation@subject_type:subject_id
//
// Example: group:972203#member@user:ahmed_456
type Tuple struct {
	ObjectType  string
	ObjectID    string
	Relation    string
	SubjectType string
	SubjectID   string
}
