package role

import (
	"context"
	"fmt"

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
		return nil, fmt.Errorf("get roles for user %d: %w", userID, err)
	}
	defer rows.Close()

	var roles []model.UserRole
	for rows.Next() {
		var r model.UserRole
		if err := rows.Scan(&r.ID, &r.UserID, &r.RoleType, &r.RoleName); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		roles = append(roles, r)
	}
	return roles, rows.Err()
}

func (s *PgStore) GetAllRoles(ctx context.Context, userID model.GlobalUserID) ([]model.UserRole, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, role_type, role_name
		FROM user_roles
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("get all roles for user %d: %w", userID, err)
	}
	defer rows.Close()

	var roles []model.UserRole
	for rows.Next() {
		var r model.UserRole
		if err := rows.Scan(&r.ID, &r.UserID, &r.RoleType, &r.RoleName); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		roles = append(roles, r)
	}
	return roles, rows.Err()
}

func (s *PgStore) AddRole(ctx context.Context, r model.UserRole) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_type, role_name)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, role_type, role_name) DO NOTHING
	`, r.UserID, r.RoleType, r.RoleName)
	if err != nil {
		return fmt.Errorf("add role %s/%s for user %d: %w", r.RoleType, r.RoleName, r.UserID, err)
	}
	return nil
}

func (s *PgStore) RemoveRole(ctx context.Context, userID model.GlobalUserID, roleType model.RoleLayer, roleName string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM user_roles
		WHERE user_id = $1 AND role_type = $2 AND role_name = $3
	`, userID, roleType, roleName)
	if err != nil {
		return fmt.Errorf("remove role %s/%s for user %d: %w", roleType, roleName, userID, err)
	}
	return nil
}

func (s *PgStore) ClearRoles(ctx context.Context, userID model.GlobalUserID, roleType model.RoleLayer) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM user_roles
		WHERE user_id = $1 AND role_type = $2
	`, userID, roleType)
	if err != nil {
		return fmt.Errorf("clear roles %s for user %d: %w", roleType, userID, err)
	}
	return nil
}

var _ Store = (*PgStore)(nil)
