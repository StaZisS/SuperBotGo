// Package university provides a sync service that writes university business
// data and authorization tuples in the same transaction.
//
// Usage:
//
//	svc := university.NewSyncService(pool)
//
//	err := svc.SyncDepartment(ctx, university.DepartmentInput{
//	    Code:        "cs",
//	    FacultyCode: "engineering",
//	    Name:        "Кафедра информатики",
//	})
package university

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"SuperBotGo/internal/authz/tuples"
)

type SyncService struct {
	pool *pgxpool.Pool
}

func NewSyncService(pool *pgxpool.Pool) *SyncService {
	return &SyncService{pool: pool}
}

func (s *SyncService) inTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ─── Hierarchy ───────────────────────────────────────────────────────────────

type FacultyInput struct {
	Code      string
	Name      string
	ShortName string
}

func (s *SyncService) SyncFaculty(ctx context.Context, in FacultyInput) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO faculties (code, name, short_name)
			VALUES ($1, $2, $3)
			ON CONFLICT (code) DO UPDATE SET name = $2, short_name = $3, updated_at = now()
		`, in.Code, in.Name, in.ShortName)
		return err
	})
}

type DepartmentInput struct {
	Code        string
	FacultyCode string
	Name        string
	ShortName   string
}

// HierarchyNodeInput is a generic input for any hierarchy entity that has
// a code, a name, and a parent entity.
type HierarchyNodeInput struct {
	Code       string
	ParentCode string
	Name       string
	// Extra columns (key=column name, value=column value).
	// Examples: "short_name" → "КИ", "degree_level" → "bachelor"
	Extra map[string]any
}

// HierarchyLevel describes one level of the organizational hierarchy.
type HierarchyLevel struct {
	Table       string // SQL table name
	ParentFK    string // FK column pointing to parent (e.g. "faculty_id")
	ParentTable string // parent SQL table name (e.g. "faculties")
	TupleType   string // object_type in tuples (e.g. "department")
	ParentTuple string // subject_type in tuples (e.g. "faculty")
}

// Predefined hierarchy levels.
var (
	LevelDepartment = HierarchyLevel{"departments", "faculty_id", "faculties", "department", "faculty"}
	LevelProgram    = HierarchyLevel{"programs", "department_id", "departments", "program", "department"}
	LevelStream     = HierarchyLevel{"streams", "program_id", "programs", "stream", "program"}
	LevelGroup      = HierarchyLevel{"study_groups", "stream_id", "streams", "group", "stream"}
	LevelSubgroup   = HierarchyLevel{"subgroups", "study_group_id", "study_groups", "subgroup", "group"}
)

// SyncHierarchyNode upserts a hierarchy entity and its parent tuple in one tx.
func (s *SyncService) SyncHierarchyNode(ctx context.Context, level HierarchyLevel, in HierarchyNodeInput) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		// Build dynamic upsert SQL.
		cols := level.ParentFK + ", code, name"
		vals := fmt.Sprintf("(SELECT id FROM %s WHERE code = $1), $2, $3", level.ParentTable)
		updates := fmt.Sprintf("%s = (SELECT id FROM %s WHERE code = $1), name = $3", level.ParentFK, level.ParentTable)
		args := []any{in.ParentCode, in.Code, in.Name}

		i := 4
		for col, val := range in.Extra {
			cols += fmt.Sprintf(", %s", col)
			vals += fmt.Sprintf(", $%d", i)
			updates += fmt.Sprintf(", %s = $%d", col, i)
			args = append(args, val)
			i++
		}

		query := fmt.Sprintf(`
			INSERT INTO %s (%s) VALUES (%s)
			ON CONFLICT (code) DO UPDATE SET %s, updated_at = now()
		`, level.Table, cols, vals, updates)

		if _, err := tx.Exec(ctx, query, args...); err != nil {
			return fmt.Errorf("upsert %s %s: %w", level.Table, in.Code, err)
		}

		return tuples.ReplaceForObject(ctx, tx, level.TupleType, in.Code, "parent", []tuples.Tuple{
			{ObjectType: level.TupleType, ObjectID: in.Code, Relation: "parent", SubjectType: level.ParentTuple, SubjectID: in.ParentCode},
		})
	})
}

// ─── Student positions ──────────────────────────────────────────────────────

type StudentPositionInput struct {
	PersonExternalID string
	ProgramCode      string
	StreamCode       string
	GroupCode        string
	Status           string
	NationalityType  string
	FundingType      string
	EducationForm    string
}

func (s *SyncService) SyncStudentPosition(ctx context.Context, in StudentPositionInput) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO student_positions (
				person_id, program_id, stream_id, study_group_id,
				status, nationality_type, funding_type, education_form
			)
			VALUES (
				(SELECT id FROM persons WHERE external_id = $1),
				(SELECT id FROM programs WHERE code = $2),
				(SELECT id FROM streams WHERE code = $3),
				(SELECT id FROM study_groups WHERE code = $4),
				$5, $6, $7, $8
			)
			ON CONFLICT ON CONSTRAINT student_positions_pkey DO NOTHING
		`, in.PersonExternalID, in.ProgramCode, in.StreamCode, in.GroupCode,
			in.Status, in.NationalityType, in.FundingType, in.EducationForm)
		if err != nil {
			return fmt.Errorf("upsert student_position for %s: %w", in.PersonExternalID, err)
		}

		if err := tuples.DeleteBySubject(ctx, tx, "user", in.PersonExternalID, "member"); err != nil {
			return err
		}

		if in.Status == "active" && in.GroupCode != "" {
			return tuples.WriteTuples(ctx, tx, []tuples.Tuple{
				{ObjectType: "group", ObjectID: in.GroupCode, Relation: "member", SubjectType: "user", SubjectID: in.PersonExternalID},
			})
		}
		return nil
	})
}

type StudentSubgroupInput struct {
	PersonExternalID string
	SubgroupCode     string
}

func (s *SyncService) SyncStudentSubgroup(ctx context.Context, in StudentSubgroupInput) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO student_subgroups (student_position_id, subgroup_id)
			VALUES (
				(SELECT sp.id FROM student_positions sp
				 JOIN persons p ON p.id = sp.person_id
				 WHERE p.external_id = $1 AND sp.status = 'active'
				 LIMIT 1),
				(SELECT id FROM subgroups WHERE code = $2)
			)
			ON CONFLICT (student_position_id, subgroup_id) DO NOTHING
		`, in.PersonExternalID, in.SubgroupCode)
		if err != nil {
			return fmt.Errorf("upsert student_subgroup %s→%s: %w", in.PersonExternalID, in.SubgroupCode, err)
		}

		return tuples.WriteTuples(ctx, tx, []tuples.Tuple{
			{ObjectType: "subgroup", ObjectID: in.SubgroupCode, Relation: "member", SubjectType: "user", SubjectID: in.PersonExternalID},
		})
	})
}

// ─── Teaching assignments ───────────────────────────────────────────────────

type TeachingAssignmentInput struct {
	PersonExternalID string
	CourseCode       string
	SemesterYear     int
	SemesterType     string // "fall" or "spring"
	StreamCode       string // one of StreamCode/GroupCode should be set
	GroupCode        string
	AssignmentType   string // lecturer, practice, supervisor, examiner
	StudentScope     string // all, foreign_only, specific_subgroup
}

func (s *SyncService) SyncTeachingAssignment(ctx context.Context, in TeachingAssignmentInput) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO teaching_assignments (
				teacher_position_id, course_id, semester_id,
				stream_id, study_group_id, assignment_type, student_scope
			)
			VALUES (
				(SELECT tp.id FROM teacher_positions tp
				 JOIN persons p ON p.id = tp.person_id
				 WHERE p.external_id = $1 LIMIT 1),
				(SELECT id FROM courses WHERE code = $2),
				(SELECT id FROM semesters WHERE year = $3 AND semester_type = $4),
				(SELECT id FROM streams WHERE code = nullif($5, '')),
				(SELECT id FROM study_groups WHERE code = nullif($6, '')),
				$7, $8
			)
		`, in.PersonExternalID, in.CourseCode,
			in.SemesterYear, in.SemesterType,
			in.StreamCode, in.GroupCode,
			in.AssignmentType, in.StudentScope)
		if err != nil {
			return fmt.Errorf("insert teaching_assignment for %s: %w", in.PersonExternalID, err)
		}

		relation := "teacher"
		if in.StudentScope == "foreign_only" {
			relation = "foreign_teacher"
		}
		objectType, objectCode := "stream", in.StreamCode
		if in.GroupCode != "" {
			objectType, objectCode = "group", in.GroupCode
		}

		return tuples.WriteTuples(ctx, tx, []tuples.Tuple{
			{ObjectType: objectType, ObjectID: objectCode, Relation: relation, SubjectType: "user", SubjectID: in.PersonExternalID},
		})
	})
}

// ─── Administrative appointments ────────────────────────────────────────────

type AdminAppointmentInput struct {
	PersonExternalID string
	AppointmentType  string // dean, dept_head, program_director, etc.
	ScopeType        string // university_wide, faculty, department, etc.
	ScopeCode        string // code of the scoped entity (empty for university_wide)
}

var appointmentRelation = map[string]string{
	"dean":                    "dean",
	"dept_head":               "head",
	"program_director":        "director",
	"stream_curator":          "curator",
	"group_curator":           "curator",
	"foreign_student_curator": "curator",
}

var scopeToObjectType = map[string]string{
	"university_wide": "university",
	"faculty":         "faculty",
	"department":      "department",
	"program":         "program",
	"stream":          "stream",
	"group":           "group",
}

func (s *SyncService) SyncAdminAppointment(ctx context.Context, in AdminAppointmentInput) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		var scopeID *int64
		if in.ScopeType != "university_wide" && in.ScopeCode != "" {
			table := scopeTypeToTable(in.ScopeType)
			if table != "" {
				var id int64
				if err := tx.QueryRow(ctx,
					"SELECT id FROM "+table+" WHERE code = $1", in.ScopeCode).Scan(&id); err == nil {
					scopeID = &id
				}
			}
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO administrative_appointments (person_id, appointment_type, scope_type, scope_id)
			VALUES (
				(SELECT id FROM persons WHERE external_id = $1),
				$2, $3, $4
			)
		`, in.PersonExternalID, in.AppointmentType, in.ScopeType, scopeID)
		if err != nil {
			return fmt.Errorf("insert admin_appointment for %s: %w", in.PersonExternalID, err)
		}

		relation := appointmentRelation[in.AppointmentType]
		if relation == "" {
			return nil
		}

		// Foreign student curator is a cross-cutting role.
		if in.AppointmentType == "foreign_student_curator" {
			return tuples.WriteTuples(ctx, tx, []tuples.Tuple{
				{ObjectType: "nationality_category", ObjectID: "foreign", Relation: relation, SubjectType: "user", SubjectID: in.PersonExternalID},
			})
		}

		objectType := scopeToObjectType[in.ScopeType]
		if objectType == "" {
			return nil
		}
		objectID := in.ScopeCode
		if in.ScopeType == "university_wide" {
			objectID = "main"
		}

		return tuples.WriteTuples(ctx, tx, []tuples.Tuple{
			{ObjectType: objectType, ObjectID: objectID, Relation: relation, SubjectType: "user", SubjectID: in.PersonExternalID},
		})
	})
}

func scopeTypeToTable(scopeType string) string {
	switch scopeType {
	case "faculty":
		return "faculties"
	case "department":
		return "departments"
	case "program":
		return "programs"
	case "stream":
		return "streams"
	case "group":
		return "study_groups"
	default:
		return ""
	}
}
