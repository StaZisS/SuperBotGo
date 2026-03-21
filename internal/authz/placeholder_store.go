package authz

import (
	"context"

	"SuperBotGo/internal/model"
)

// PlaceholderStore is an in-memory stub used when PostgreSQL is unavailable.
// All checks return permissive defaults (no tuples, no policies).
type PlaceholderStore struct{}

func NewPlaceholderStore() *PlaceholderStore { return &PlaceholderStore{} }

func (s *PlaceholderStore) GetRoles(_ context.Context, _ model.GlobalUserID, _ model.RoleLayer) ([]model.UserRole, error) {
	return nil, nil
}

func (s *PlaceholderStore) GetAllRoleNames(_ context.Context, _ model.GlobalUserID) ([]string, error) {
	return nil, nil
}

func (s *PlaceholderStore) GetAllUserRelations(_ context.Context, _ string) ([]RelationEntry, error) {
	return nil, nil
}

func (s *PlaceholderStore) GetMemberGroups(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

func (s *PlaceholderStore) GetCommandPolicy(_ context.Context, _, _ string) (bool, string, bool, error) {
	return true, "", false, nil
}

func (s *PlaceholderStore) GetExternalID(_ context.Context, _ model.GlobalUserID) (string, error) {
	return "", nil
}

func (s *PlaceholderStore) GetUserChannelAndLocale(_ context.Context, _ model.GlobalUserID) (string, string, error) {
	return "", "", nil
}

func (s *PlaceholderStore) GetDistinctRoleNames(_ context.Context) []string {
	return nil
}

var _ Store = (*PlaceholderStore)(nil)
