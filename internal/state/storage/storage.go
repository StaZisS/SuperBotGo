package storage

import (
	"context"

	"SuperBotGo/internal/model"
)

// DialogStorage defines the persistence interface for dialog state.
// Implementations may use Redis, an in-memory map, a database, etc.
type DialogStorage interface {
	// Save persists the dialog state for the given user, overwriting any
	// previously stored state.
	Save(ctx context.Context, userID model.GlobalUserID, state model.DialogState) error

	// Load retrieves the dialog state for the given user.
	// Returns (nil, nil) if no state exists.
	Load(ctx context.Context, userID model.GlobalUserID) (*model.DialogState, error)

	// Delete removes the dialog state for the given user.
	// It is not an error to delete a non-existent state.
	Delete(ctx context.Context, userID model.GlobalUserID) error
}
