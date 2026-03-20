package state

import (
	"context"
	"fmt"
	"sync"

	"SuperBotGo/internal/model"
	"SuperBotGo/internal/state/storage"
)

// Manager orchestrates command dialogs. It holds registered command definitions,
// delegates state management to a StateHandler, and uses a DialogStorage
// backend for persistence.
type Manager struct {
	storage  storage.DialogStorage
	commands map[string]*CommandDefinition
	handlers map[string]StateHandler
	mu       sync.RWMutex
}

// NewManager creates a new state Manager with the given storage backend.
func NewManager(store storage.DialogStorage) *Manager {
	return &Manager{
		storage:  store,
		commands: make(map[string]*CommandDefinition),
		handlers: make(map[string]StateHandler),
	}
}

// RegisterCommand registers a DSL command definition and creates a
// DslStateHandler for it.
func (m *Manager) RegisterCommand(def *CommandDefinition) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commands[def.Name] = def
	m.handlers[def.Name] = NewDslStateHandler(def)
}

// RegisterHandler registers a custom StateHandler under the given name.
func (m *Manager) RegisterHandler(name string, handler StateHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[name] = handler
}

// IsCommandImmediate returns true if the command has no dialog steps
// and should be executed immediately without user interaction.
func (m *Manager) IsCommandImmediate(commandName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	def, ok := m.commands[commandName]
	if !ok {
		return false
	}
	return def.IsComplete(nil)
}

// StartCommand begins a new command dialog for the given user. It creates a
// fresh state, persists it, and returns the first step's prompt message.
func (m *Manager) StartCommand(ctx context.Context, userID model.GlobalUserID, commandName string, locale string) (model.Message, error) {
	m.mu.RLock()
	handler, ok := m.handlers[commandName]
	m.mu.RUnlock()
	if !ok {
		return model.Message{}, fmt.Errorf("%w: %s", ErrCommandNotFound, commandName)
	}

	state, err := handler.CreateNewState(commandName)
	if err != nil {
		return model.Message{}, fmt.Errorf("creating state for %s: %w", commandName, err)
	}

	msg := handler.BuildStepMessage(state, locale)

	ds := handler.PersistState(state)
	if err := m.storage.Save(ctx, userID, ds); err != nil {
		return model.Message{}, fmt.Errorf("saving state: %w", err)
	}

	return msg, nil
}

// ProcessInput handles user input within an active dialog. It restores state,
// processes the input, and either advances to the next step or completes the
// command. When complete, it returns a non-nil *model.CommandRequest containing
// the collected parameters.
func (m *Manager) ProcessInput(ctx context.Context, userID model.GlobalUserID, input model.UserInput, locale string) (model.Message, *model.CommandRequest, error) {
	ds, err := m.storage.Load(ctx, userID)
	if err != nil {
		return model.Message{}, nil, fmt.Errorf("loading state: %w", err)
	}
	if ds == nil {
		return model.Message{}, nil, ErrNoActiveDialog
	}

	m.mu.RLock()
	handler, ok := m.handlers[ds.CommandName]
	m.mu.RUnlock()
	if !ok {
		return model.Message{}, nil, fmt.Errorf("%w: %s", ErrCommandNotFound, ds.CommandName)
	}

	state, err := handler.RestoreState(*ds)
	if err != nil {
		return model.Message{}, nil, fmt.Errorf("restoring state: %w", err)
	}

	nextState, outcome, err := handler.ProcessInput(userID, state, input)
	if err != nil {
		return model.Message{}, nil, fmt.Errorf("processing input: %w", err)
	}

	msg := handler.BuildStepMessage(nextState, locale)
	outcome.Message = msg

	if outcome.IsComplete {
		if delErr := m.storage.Delete(ctx, userID); delErr != nil {
			return model.Message{}, nil, fmt.Errorf("deleting state: %w", delErr)
		}
		cmdReq := &model.CommandRequest{
			UserID:      userID,
			CommandName: outcome.CommandName,
			Params:      outcome.Params,
			Locale:      locale,
		}
		return msg, cmdReq, nil
	}

	persistedDS := handler.PersistState(nextState)
	if err := m.storage.Save(ctx, userID, persistedDS); err != nil {
		return model.Message{}, nil, fmt.Errorf("saving state: %w", err)
	}

	return msg, nil, nil
}

// HasActiveDialog checks whether the user currently has an active command dialog.
func (m *Manager) HasActiveDialog(ctx context.Context, userID model.GlobalUserID) bool {
	ds, err := m.storage.Load(ctx, userID)
	return err == nil && ds != nil
}

// CancelCommand removes the active dialog for the user.
func (m *Manager) CancelCommand(ctx context.Context, userID model.GlobalUserID) error {
	return m.storage.Delete(ctx, userID)
}
