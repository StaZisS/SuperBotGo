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
