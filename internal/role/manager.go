package role

import (
	"log/slog"
)

type Manager struct {
	store  Store
	logger *slog.Logger
}

func NewManager(store Store, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{store: store, logger: logger}
}
