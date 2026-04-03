package university

import (
	"context"
	"fmt"

	"SuperBotGo/internal/model"

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
