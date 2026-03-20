package policy

import (
	"context"
	"fmt"

	"github.com/expr-lang/expr"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"SuperBotGo/internal/model"
)

// UserContext — контекст пользователя, доступный в выражениях как `user.*`.
type UserContext struct {
	ID              int64    `expr:"id"`
	ExternalID      string   `expr:"external_id"`
	NationalityType string   `expr:"nationality_type"`
	FundingType     string   `expr:"funding_type"`
	EducationForm   string   `expr:"education_form"`
	Groups          []string `expr:"groups"`
	Roles           []string `expr:"roles"`
	PrimaryChannel  string   `expr:"primary_channel"`
	Locale          string   `expr:"locale"`
}

// Env — окружение для вычисления выражений.
type Env struct {
	User UserContext `expr:"user"`

	// Функции, доступные в выражениях
	Check      func(relation, objectType, objectID string) bool `expr:"check"`
	IsMember   func(objectType, objectID string) bool           `expr:"is_member"`
	HasRole    func(roleName string) bool                       `expr:"has_role"`
	HasAnyRole func(roleNames ...string) bool                   `expr:"has_any_role"`
}

// Evaluator вычисляет policy-выражения с контекстом пользователя.
type Evaluator struct {
	pool *pgxpool.Pool
}

func NewEvaluator(pool *pgxpool.Pool) *Evaluator {
	return &Evaluator{pool: pool}
}

// Evaluate вычисляет выражение для данного пользователя. Возвращает true если доступ разрешён.
func (e *Evaluator) Evaluate(ctx context.Context, expression string, userID model.GlobalUserID) (bool, error) {
	env, err := e.buildEnv(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("build env: %w", err)
	}

	program, err := expr.Compile(expression, expr.Env(env), expr.AsBool())
	if err != nil {
		return false, fmt.Errorf("compile expression: %w", err)
	}

	result, err := expr.Run(program, env)
	if err != nil {
		return false, fmt.Errorf("run expression: %w", err)
	}

	b, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("expression must return bool, got %T", result)
	}
	return b, nil
}

func (e *Evaluator) buildEnv(ctx context.Context, userID model.GlobalUserID) (Env, error) {
	uc := UserContext{ID: int64(userID)}

	// Загружаем данные person
	var extID *string
	err := e.pool.QueryRow(ctx, `
		SELECT external_id FROM persons WHERE global_user_id = $1
	`, userID).Scan(&extID)
	if err == nil && extID != nil {
		uc.ExternalID = *extID
	} else if err != nil && err != pgx.ErrNoRows {
		return Env{}, err
	}

	// Атрибуты студента (берём первую активную позицию)
	if uc.ExternalID != "" {
		var natType, fundType, eduForm *string
		_ = e.pool.QueryRow(ctx, `
			SELECT sp.nationality_type, sp.funding_type, sp.education_form
			FROM student_positions sp
			JOIN persons p ON p.id = sp.person_id
			WHERE p.external_id = $1 AND sp.status = 'active'
			LIMIT 1
		`, uc.ExternalID).Scan(&natType, &fundType, &eduForm)
		if natType != nil {
			uc.NationalityType = *natType
		}
		if fundType != nil {
			uc.FundingType = *fundType
		}
		if eduForm != nil {
			uc.EducationForm = *eduForm
		}
	}

	// Группы, в которых состоит пользователь
	if uc.ExternalID != "" {
		rows, err := e.pool.Query(ctx, `
			SELECT object_id FROM authorization_tuples
			WHERE subject_type = 'user' AND subject_id = $1 AND relation = 'member'
			  AND object_type = 'group'
		`, uc.ExternalID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var g string
				if rows.Scan(&g) == nil {
					uc.Groups = append(uc.Groups, g)
				}
			}
		}
	}

	// Роли пользователя
	{
		rows, err := e.pool.Query(ctx, `
			SELECT role_name FROM user_roles WHERE user_id = $1
		`, userID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var r string
				if rows.Scan(&r) == nil {
					uc.Roles = append(uc.Roles, r)
				}
			}
		}
	}

	// Данные из global_users
	{
		var ch, loc *string
		_ = e.pool.QueryRow(ctx, `
			SELECT primary_channel, locale FROM global_users WHERE id = $1
		`, userID).Scan(&ch, &loc)
		if ch != nil {
			uc.PrimaryChannel = *ch
		}
		if loc != nil {
			uc.Locale = *loc
		}
	}

	// SQL: проверяет, что пользователь имеет relation на объекте ИЛИ на чём-то внутри объекта.
	// Т.е. ahmed member группы 972203, а 972203 внутри faculty:engineering → check("member", "faculty", "engineering") = true
	const belongsToSQL = `
		WITH RECURSIVE
		-- Все объекты, где у пользователя есть данная relation
		user_objects AS (
			SELECT object_type AS ot, object_id AS oid
			FROM authorization_tuples
			WHERE subject_type = 'user' AND subject_id = $1 AND relation = $2
		),
		-- От каждого такого объекта идём вверх по parent-цепочке
		ancestors AS (
			SELECT ot, oid, 0 AS depth FROM user_objects
			UNION ALL
			SELECT at.subject_type, at.subject_id, a.depth + 1
			FROM ancestors a
			JOIN authorization_tuples at
				ON at.object_type = a.ot AND at.object_id = a.oid AND at.relation = 'parent'
			WHERE a.depth < 10
		)
		SELECT EXISTS(
			SELECT 1 FROM ancestors WHERE ot = $3 AND oid = $4
		)`

	env := Env{
		User: uc,

		// check(relation, object_type, object_id) — пользователь имеет relation внутри указанного объекта
		// Пример: check("member", "faculty", "engineering") — ahmed member группы внутри этого факультета
		Check: func(relation, objectType, objectID string) bool {
			if uc.ExternalID == "" {
				return false
			}
			var ok bool
			_ = e.pool.QueryRow(ctx, belongsToSQL,
				uc.ExternalID, relation, objectType, objectID,
			).Scan(&ok)
			return ok
		},

		// is_member(object_type, object_id) — shortcut для check("member", ...)
		IsMember: func(objectType, objectID string) bool {
			if uc.ExternalID == "" {
				return false
			}
			var ok bool
			_ = e.pool.QueryRow(ctx, belongsToSQL,
				uc.ExternalID, "member", objectType, objectID,
			).Scan(&ok)
			return ok
		},

		// has_role(role_name) — проверяет user_roles
		HasRole: func(roleName string) bool {
			for _, r := range uc.Roles {
				if r == roleName {
					return true
				}
			}
			return false
		},

		// has_any_role("ADMIN", "MODERATOR") — любая из указанных
		HasAnyRole: func(roleNames ...string) bool {
			for _, rn := range roleNames {
				for _, r := range uc.Roles {
					if r == rn {
						return true
					}
				}
			}
			return false
		},
	}

	return env, nil
}
