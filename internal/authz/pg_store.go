package authz

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"SuperBotGo/internal/model"
)

type PgStore struct {
	pool *pgxpool.Pool
}

func NewPgStore(pool *pgxpool.Pool) *PgStore {
	return &PgStore{pool: pool}
}

func (s *PgStore) GetRoles(ctx context.Context, userID model.GlobalUserID, roleType model.RoleLayer) ([]model.UserRole, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, role_type, role_name
		FROM user_roles
		WHERE user_id = $1 AND role_type = $2
	`, userID, roleType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []model.UserRole
	for rows.Next() {
		var r model.UserRole
		if err := rows.Scan(&r.ID, &r.UserID, &r.RoleType, &r.RoleName); err != nil {
			return nil, err
		}
		roles = append(roles, r)
	}
	return roles, rows.Err()
}

func (s *PgStore) GetAllRoleNames(ctx context.Context, userID model.GlobalUserID) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT role_name FROM user_roles WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// allUserRelationsSQL returns every (relation, objectType, objectID) a user
// has — including those inherited through the parent chain.
// One query, no N+1.
const allUserRelationsSQL = `
WITH RECURSIVE
direct AS (
	SELECT relation, object_type, object_id
	FROM authorization_tuples
	WHERE subject_type = 'user' AND subject_id = $1
	  AND relation != 'parent'
),
expanded AS (
	SELECT relation, object_type AS ot, object_id AS oid, 0 AS depth
	FROM direct
	UNION ALL
	SELECT e.relation, at.subject_type, at.subject_id, e.depth + 1
	FROM expanded e
	JOIN authorization_tuples at
		ON at.object_type = e.ot AND at.object_id = e.oid AND at.relation = 'parent'
	WHERE e.depth < 10
)
SELECT DISTINCT relation, ot, oid FROM expanded`

func (s *PgStore) GetAllUserRelations(ctx context.Context, subjectID string) ([]RelationEntry, error) {
	rows, err := s.pool.Query(ctx, allUserRelationsSQL, subjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []RelationEntry
	for rows.Next() {
		var e RelationEntry
		if err := rows.Scan(&e.Relation, &e.ObjectType, &e.ObjectID); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *PgStore) GetMemberGroups(ctx context.Context, subjectID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT object_id FROM authorization_tuples
		WHERE subject_type = 'user' AND subject_id = $1 AND relation = 'member'
		  AND object_type = 'group'
	`, subjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []string
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

func (s *PgStore) GetCommandPolicy(ctx context.Context, pluginID, commandName string) (bool, string, bool, error) {
	var enabled bool
	var policyExpr *string
	err := s.pool.QueryRow(ctx, `
		SELECT enabled, policy_expression FROM plugin_command_settings
		WHERE plugin_id = $1 AND command_name = $2
	`, pluginID, commandName).Scan(&enabled, &policyExpr)

	if err == pgx.ErrNoRows {
		return true, "", false, nil
	}
	if err != nil {
		return false, "", false, err
	}

	expr := ""
	if policyExpr != nil {
		expr = *policyExpr
	}
	return enabled, expr, true, nil
}

func (s *PgStore) GetExternalID(ctx context.Context, userID model.GlobalUserID) (string, error) {
	var extID *string
	err := s.pool.QueryRow(ctx, `
		SELECT external_id FROM persons WHERE global_user_id = $1
	`, userID).Scan(&extID)

	if err == pgx.ErrNoRows || extID == nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return *extID, nil
}

func (s *PgStore) GetUserChannelAndLocale(ctx context.Context, userID model.GlobalUserID) (string, string, error) {
	var ch, loc *string
	err := s.pool.QueryRow(ctx, `
		SELECT primary_channel, locale FROM global_users WHERE id = $1
	`, userID).Scan(&ch, &loc)

	if err == pgx.ErrNoRows {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}

	primaryChannel := ""
	locale := ""
	if ch != nil {
		primaryChannel = *ch
	}
	if loc != nil {
		locale = *loc
	}
	return primaryChannel, locale, nil
}

func (s *PgStore) GetDistinctRoleNames(ctx context.Context) []string {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT role_name FROM user_roles ORDER BY role_name
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if rows.Scan(&name) == nil {
			names = append(names, name)
		}
	}
	return names
}

var _ Store = (*PgStore)(nil)
