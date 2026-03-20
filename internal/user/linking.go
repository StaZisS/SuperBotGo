package user

import (
	"context"
	"time"

	"SuperBotGo/internal/model"
)

type LinkCodeStorage interface {
	SaveCode(ctx context.Context, code string, userID model.GlobalUserID, ttl time.Duration) error
	GetUserID(ctx context.Context, code string) (model.GlobalUserID, error)
	DeleteCode(ctx context.Context, code string) error
}

type PlaceholderLinkCodeStorage struct {
	codes map[string]model.GlobalUserID
}

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
