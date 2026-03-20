package storage

import (
	"context"

	"SuperBotGo/internal/model"
)

type DialogStorage interface {
	Save(ctx context.Context, userID model.GlobalUserID, state model.DialogState) error

	Load(ctx context.Context, userID model.GlobalUserID) (*model.DialogState, error)

	Delete(ctx context.Context, userID model.GlobalUserID) error
}
