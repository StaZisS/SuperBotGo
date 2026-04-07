package university

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"SuperBotGo/internal/authz/outbox"
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
	defer tx.Rollback(ctx)

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type PersonInput struct {
	ExternalID string
	LastName   string
	FirstName  string
	MiddleName string
	Email      string
	Phone      string
}

func (s *SyncService) SyncPerson(ctx context.Context, in PersonInput) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO persons (external_id, last_name, first_name, middle_name, email, phone)
			VALUES ($1, $2, $3, nullif($4, ''), nullif($5, ''), nullif($6, ''))
			ON CONFLICT (external_id) DO UPDATE SET
				last_name = $2, first_name = $3, middle_name = nullif($4, ''),
				email = nullif($5, ''), phone = nullif($6, ''), updated_at = now()
		`, in.ExternalID, in.LastName, in.FirstName, in.MiddleName, in.Email, in.Phone)
		if err != nil {
			return err
		}

		// Auto-link: if a global_user with matching tsu_accounts_id exists,
		// set persons.global_user_id automatically.
		_, err = tx.Exec(ctx, `
			UPDATE persons
			SET global_user_id = (SELECT id FROM global_users WHERE tsu_accounts_id = $1),
			    updated_at = now()
			WHERE external_id = $1
			  AND global_user_id IS NULL
			  AND EXISTS (SELECT 1 FROM global_users WHERE tsu_accounts_id = $1)
		`, in.ExternalID)
		return err
	})
}

type CourseInput struct {
	Code string
	Name string
}

func (s *SyncService) SyncCourse(ctx context.Context, in CourseInput) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO courses (code, name)
			VALUES ($1, $2)
			ON CONFLICT (code) DO UPDATE SET name = $2, updated_at = now()
		`, in.Code, in.Name)
		return err
	})
}

type SemesterInput struct {
	Year         int
	SemesterType string
}

func (s *SyncService) SyncSemester(ctx context.Context, in SemesterInput) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO semesters (year, semester_type)
			VALUES ($1, $2)
			ON CONFLICT (year, semester_type) DO NOTHING
		`, in.Year, in.SemesterType)
		return err
	})
}

type TeacherPositionInput struct {
	PersonExternalID string
	DepartmentCode   string
	PositionTitle    string
	EmploymentType   string
}

func (s *SyncService) SyncTeacherPosition(ctx context.Context, in TeacherPositionInput) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO teacher_positions (person_id, department_id, position_title, employment_type)
			VALUES (
				(SELECT id FROM persons WHERE external_id = $1),
				(SELECT id FROM departments WHERE code = $2),
				$3, $4
			)
			ON CONFLICT DO NOTHING
		`, in.PersonExternalID, in.DepartmentCode, in.PositionTitle, in.EmploymentType)
		return err
	})
}

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

type HierarchyNodeInput struct {
	Code       string
	ParentCode string
	Name       string
	Extra      map[string]any
}

type HierarchyLevel struct {
	Table       string
	ParentFK    string
	ParentTable string
	TupleType   string
	ParentTuple string
}

var (
	LevelDepartment = HierarchyLevel{"departments", "faculty_id", "faculties", "department", "faculty"}
	LevelProgram    = HierarchyLevel{"programs", "department_id", "departments", "program", "department"}
	LevelStream     = HierarchyLevel{"streams", "program_id", "programs", "stream", "program"}
	LevelGroup      = HierarchyLevel{"study_groups", "stream_id", "streams", "study_group", "stream"}
	LevelSubgroup   = HierarchyLevel{"subgroups", "study_group_id", "study_groups", "subgroup", "study_group"}
)

func (s *SyncService) SyncHierarchyNode(ctx context.Context, level HierarchyLevel, in HierarchyNodeInput) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
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

		return outbox.EnqueueReplace(ctx, tx, level.TupleType, in.Code, "parent", []tuples.Tuple{
			{ObjectType: level.TupleType, ObjectID: in.Code, Relation: "parent", SubjectType: level.ParentTuple, SubjectID: in.ParentCode},
		})
	})
}

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
		if _, err := tx.Exec(ctx, `
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
			in.Status, in.NationalityType, in.FundingType, in.EducationForm); err != nil {
			return fmt.Errorf("upsert student_position for %s: %w", in.PersonExternalID, err)
		}

		if err := outbox.EnqueueDeleteBySubject(ctx, tx, "user", in.PersonExternalID, "member"); err != nil {
			return err
		}

		if in.Status == "active" && in.GroupCode != "" {
			return outbox.EnqueueTouch(ctx, tx, []tuples.Tuple{
				{ObjectType: "study_group", ObjectID: in.GroupCode, Relation: "member", SubjectType: "user", SubjectID: in.PersonExternalID},
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
		if _, err := tx.Exec(ctx, `
			INSERT INTO student_subgroups (student_position_id, subgroup_id)
			VALUES (
				(SELECT sp.id FROM student_positions sp
				 JOIN persons p ON p.id = sp.person_id
				 WHERE p.external_id = $1 AND sp.status = 'active'
				 LIMIT 1),
				(SELECT id FROM subgroups WHERE code = $2)
			)
			ON CONFLICT (student_position_id, subgroup_id) DO NOTHING
		`, in.PersonExternalID, in.SubgroupCode); err != nil {
			return fmt.Errorf("upsert student_subgroup %s->%s: %w", in.PersonExternalID, in.SubgroupCode, err)
		}

		return outbox.EnqueueTouch(ctx, tx, []tuples.Tuple{
			{ObjectType: "subgroup", ObjectID: in.SubgroupCode, Relation: "member", SubjectType: "user", SubjectID: in.PersonExternalID},
		})
	})
}

type TeachingAssignmentInput struct {
	PersonExternalID string
	CourseCode       string
	SemesterYear     int
	SemesterType     string
	StreamCode       string
	GroupCode        string
	AssignmentType   string
	StudentScope     string
}

func (s *SyncService) SyncTeachingAssignment(ctx context.Context, in TeachingAssignmentInput) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
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
			in.AssignmentType, in.StudentScope); err != nil {
			return fmt.Errorf("insert teaching_assignment for %s: %w", in.PersonExternalID, err)
		}

		relation := "teacher"
		if in.StudentScope == "foreign_only" {
			relation = "foreign_teacher"
		}
		objectType, objectCode := "stream", in.StreamCode
		if in.GroupCode != "" {
			objectType, objectCode = "study_group", in.GroupCode
		}

		return outbox.EnqueueTouch(ctx, tx, []tuples.Tuple{
			{ObjectType: objectType, ObjectID: objectCode, Relation: relation, SubjectType: "user", SubjectID: in.PersonExternalID},
		})
	})
}

type AdminAppointmentInput struct {
	PersonExternalID string
	AppointmentType  string
	ScopeType        string
	ScopeCode        string
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
	"group":           "study_group",
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

		if _, err := tx.Exec(ctx, `
			INSERT INTO administrative_appointments (person_id, appointment_type, scope_type, scope_id)
			VALUES (
				(SELECT id FROM persons WHERE external_id = $1),
				$2, $3, $4
			)
		`, in.PersonExternalID, in.AppointmentType, in.ScopeType, scopeID); err != nil {
			return fmt.Errorf("insert admin_appointment for %s: %w", in.PersonExternalID, err)
		}

		relation := appointmentRelation[in.AppointmentType]
		if relation == "" {
			return nil
		}

		if in.AppointmentType == "foreign_student_curator" {
			return outbox.EnqueueTouch(ctx, tx, []tuples.Tuple{
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

		return outbox.EnqueueTouch(ctx, tx, []tuples.Tuple{
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
