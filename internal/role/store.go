package role

import (
	"context"
	"sync"

	"SuperBotGo/internal/model"
)

type Store interface {
	GetRoles(ctx context.Context, userID model.GlobalUserID, roleType model.RoleLayer) ([]model.UserRole, error)
	GetAllRoles(ctx context.Context, userID model.GlobalUserID) ([]model.UserRole, error)
	AddRole(ctx context.Context, role model.UserRole) error
	RemoveRole(ctx context.Context, userID model.GlobalUserID, roleType model.RoleLayer, roleName string) error
	ClearRoles(ctx context.Context, userID model.GlobalUserID, roleType model.RoleLayer) error
}

type PlaceholderStore struct {
	mu    sync.RWMutex
	roles map[model.GlobalUserID][]model.UserRole
}

func NewPlaceholderStore() *PlaceholderStore {
	return &PlaceholderStore{
		roles: make(map[model.GlobalUserID][]model.UserRole),
	}
}

func (s *PlaceholderStore) GetRoles(_ context.Context, userID model.GlobalUserID, roleType model.RoleLayer) ([]model.UserRole, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []model.UserRole
	for _, r := range s.roles[userID] {
		if r.RoleType == roleType {
			result = append(result, r)
		}
	}
	return result, nil
}

func (s *PlaceholderStore) GetAllRoles(_ context.Context, userID model.GlobalUserID) ([]model.UserRole, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	roles := s.roles[userID]
	result := make([]model.UserRole, len(roles))
	copy(result, roles)
	return result, nil
}

func (s *PlaceholderStore) AddRole(_ context.Context, role model.UserRole) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roles[role.UserID] = append(s.roles[role.UserID], role)
	return nil
}

func (s *PlaceholderStore) RemoveRole(_ context.Context, userID model.GlobalUserID, roleType model.RoleLayer, roleName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	roles := s.roles[userID]
	filtered := roles[:0]
	for _, r := range roles {
		if !(r.RoleType == roleType && r.RoleName == roleName) {
			filtered = append(filtered, r)
		}
	}
	s.roles[userID] = filtered
	return nil
}

func (s *PlaceholderStore) ClearRoles(_ context.Context, userID model.GlobalUserID, roleType model.RoleLayer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	roles := s.roles[userID]
	filtered := roles[:0]
	for _, r := range roles {
		if r.RoleType != roleType {
			filtered = append(filtered, r)
		}
	}
	s.roles[userID] = filtered
	return nil
}

var _ Store = (*PlaceholderStore)(nil)
