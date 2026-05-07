package university

import (
	"context"
	"fmt"
	"strings"

	"SuperBotGo/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgStudentResolver resolves a university hierarchy scope + target ID
// to a list of global user IDs of active students.
type PgStudentResolver struct {
	pool *pgxpool.Pool
}

func NewPgStudentResolver(pool *pgxpool.Pool) *PgStudentResolver {
	return &PgStudentResolver{pool: pool}
}

// ResolveStudentUsers returns global_user_ids of all active students
// within the given hierarchy scope.
func (r *PgStudentResolver) ResolveStudentUsers(ctx context.Context, scope string, targetID int64) ([]model.GlobalUserID, error) {
	query, err := r.queryForScope(scope)
	if err != nil {
		return nil, err
	}

	rows, err := r.pool.Query(ctx, query, targetID)
	if err != nil {
		return nil, fmt.Errorf("resolve students %s/%d: %w", scope, targetID, err)
	}
	defer rows.Close()

	var ids []model.GlobalUserID
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan global_user_id: %w", err)
		}
		ids = append(ids, model.GlobalUserID(id))
	}
	return ids, rows.Err()
}

// ResolveTeacherUser returns the linked global_user_id for an active teacher.
func (r *PgStudentResolver) ResolveTeacherUser(ctx context.Context, ref model.TeacherRef) (model.GlobalUserID, error) {
	query, arg, label, err := teacherQuery(ref)
	if err != nil {
		return 0, err
	}

	var id int64
	if err := r.pool.QueryRow(ctx, query, arg).Scan(&id); err != nil {
		if err == pgx.ErrNoRows {
			return 0, fmt.Errorf("teacher %s is not found, inactive, or not linked to a bot user", label)
		}
		return 0, fmt.Errorf("resolve teacher %s: %w", label, err)
	}
	return model.GlobalUserID(id), nil
}

func teacherQuery(ref model.TeacherRef) (query string, arg any, label string, err error) {
	if ref.TeacherPositionID > 0 {
		return `SELECT p.global_user_id
			FROM teacher_positions tp
			JOIN persons p ON p.id = tp.person_id
			WHERE tp.id = $1
			  AND tp.status = 'active'
			  AND p.global_user_id IS NOT NULL`,
			ref.TeacherPositionID,
			fmt.Sprintf("position_id=%d", ref.TeacherPositionID),
			nil
	}

	if ref.PersonID > 0 {
		return `SELECT p.global_user_id
			FROM persons p
			WHERE p.id = $1
			  AND p.global_user_id IS NOT NULL
			  AND EXISTS (
			      SELECT 1 FROM teacher_positions tp
			      WHERE tp.person_id = p.id AND tp.status = 'active'
			  )`,
			ref.PersonID,
			fmt.Sprintf("person_id=%d", ref.PersonID),
			nil
	}

	if externalID := strings.TrimSpace(ref.ExternalID); externalID != "" {
		return `SELECT p.global_user_id
			FROM persons p
			WHERE p.external_id = $1
			  AND p.global_user_id IS NOT NULL
			  AND EXISTS (
			      SELECT 1 FROM teacher_positions tp
			      WHERE tp.person_id = p.id AND tp.status = 'active'
			  )`,
			externalID,
			fmt.Sprintf("external_id=%q", externalID),
			nil
	}

	return "", nil, "", fmt.Errorf("teacher reference is empty")
}

func (r *PgStudentResolver) queryForScope(scope string) (string, error) {
	switch scope {
	case "subgroup":
		return `SELECT DISTINCT p.global_user_id
			FROM student_subgroups ss
			JOIN student_positions sp ON sp.id = ss.student_position_id
			JOIN persons p ON p.id = sp.person_id
			WHERE ss.subgroup_id = $1
			  AND sp.status = 'active'
			  AND p.global_user_id IS NOT NULL`, nil
	case "group":
		return `SELECT DISTINCT p.global_user_id
			FROM student_positions sp
			JOIN persons p ON p.id = sp.person_id
			WHERE sp.study_group_id = $1
			  AND sp.status = 'active'
			  AND p.global_user_id IS NOT NULL`, nil
	case "stream":
		return `SELECT DISTINCT p.global_user_id
			FROM student_positions sp
			JOIN persons p ON p.id = sp.person_id
			WHERE sp.stream_id = $1
			  AND sp.status = 'active'
			  AND p.global_user_id IS NOT NULL`, nil
	case "program":
		return `SELECT DISTINCT p.global_user_id
			FROM student_positions sp
			JOIN persons p ON p.id = sp.person_id
			WHERE sp.program_id = $1
			  AND sp.status = 'active'
			  AND p.global_user_id IS NOT NULL`, nil
	case "department":
		return `SELECT DISTINCT p.global_user_id
			FROM student_positions sp
			JOIN programs pr ON pr.id = sp.program_id
			JOIN persons p ON p.id = sp.person_id
			WHERE pr.department_id = $1
			  AND sp.status = 'active'
			  AND p.global_user_id IS NOT NULL`, nil
	case "faculty":
		return `SELECT DISTINCT p.global_user_id
			FROM student_positions sp
			JOIN programs pr ON pr.id = sp.program_id
			JOIN departments d ON d.id = pr.department_id
			JOIN persons p ON p.id = sp.person_id
			WHERE d.faculty_id = $1
			  AND sp.status = 'active'
			  AND p.global_user_id IS NOT NULL`, nil
	default:
		return "", fmt.Errorf("unknown student scope: %q", scope)
	}
}
