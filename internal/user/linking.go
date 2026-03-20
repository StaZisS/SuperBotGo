package user

import (
	"context"
	"time"

	"SuperBotGo/internal/model"
)

// LinkCodeStorage persists temporary link codes for account linking.
type LinkCodeStorage interface {
	// SaveCode stores a link code associated with a user, with a TTL.
	SaveCode(ctx context.Context, code string, userID model.GlobalUserID, ttl time.Duration) error
	// GetUserID returns the user ID associated with a code, or 0 if not found.
	GetUserID(ctx context.Context, code string) (model.GlobalUserID, error)
	// DeleteCode removes a link code.
	DeleteCode(ctx context.Context, code string) error
}

// PlaceholderLinkCodeStorage is an in-memory placeholder for LinkCodeStorage.
// In production this will be backed by Redis.
type PlaceholderLinkCodeStorage struct {
	// codes maps code -> userID (ignores TTL for simplicity).
	codes map[string]model.GlobalUserID
}

// NewPlaceholderLinkCodeStorage creates a new placeholder link code storage.
func NewPlaceholderLinkCodeStorage() *PlaceholderLinkCodeStorage {
	return &PlaceholderLinkCodeStorage{
		codes: make(map[string]model.GlobalUserID),
	}
}

func (s *PlaceholderLinkCodeStorage) SaveCode(_ context.Context, code string, userID model.GlobalUserID, _ time.Duration) error {
	s.codes[code] = userID
	return nil
}

func (s *PlaceholderLinkCodeStorage) GetUserID(_ context.Context, code string) (model.GlobalUserID, error) {
	id, ok := s.codes[code]
	if !ok {
		return 0, nil
	}
	return id, nil
}

func (s *PlaceholderLinkCodeStorage) DeleteCode(_ context.Context, code string) error {
	delete(s.codes, code)
	return nil
}

var _ LinkCodeStorage = (*PlaceholderLinkCodeStorage)(nil)
