package state

import (
	"context"
	"fmt"
	"sync"

	"SuperBotGo/internal/model"
	"SuperBotGo/internal/state/storage"
)

type Manager struct {
	storage  storage.DialogStorage
	commands map[string]*CommandDefinition
	handlers map[string]StateHandler
	mu       sync.RWMutex
}

func NewManager(store storage.DialogStorage) *Manager {
	return &Manager{
		storage:  store,
		commands: make(map[string]*CommandDefinition),
		handlers: make(map[string]StateHandler),
	}
}

func (m *Manager) RegisterCommand(def *CommandDefinition) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commands[def.Name] = def
	m.handlers[def.Name] = NewDslStateHandler(def)
}

func (m *Manager) UnregisterCommand(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.commands, name)
	delete(m.handlers, name)
}

func (m *Manager) RegisterHandler(name string, handler StateHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[name] = handler
}

func (m *Manager) IsCommandImmediate(commandName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	def, ok := m.commands[commandName]
	if !ok {
		return false
	}
	return def.IsComplete(nil)
}

func (m *Manager) StartCommand(ctx context.Context, userID model.GlobalUserID, chatID string, commandName string, locale string) (model.Message, error) {
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
	ds.ChatID = chatID
	if err := m.storage.Save(ctx, userID, ds); err != nil {
		return model.Message{}, fmt.Errorf("saving state: %w", err)
	}

	return msg, nil
}

func (m *Manager) ProcessInput(ctx context.Context, userID model.GlobalUserID, chatID string, input model.UserInput, locale string) (model.Message, *model.CommandRequest, error) {
	ds, err := m.storage.Load(ctx, userID)
	if err != nil {
		return model.Message{}, nil, fmt.Errorf("loading state: %w", err)
	}
	if ds == nil {
		return model.Message{}, nil, ErrNoActiveDialog
	}
	if ds.ChatID != "" && ds.ChatID != chatID {
		return model.Message{}, nil, nil
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
	persistedDS.ChatID = ds.ChatID
	if err := m.storage.Save(ctx, userID, persistedDS); err != nil {
		return model.Message{}, nil, fmt.Errorf("saving state: %w", err)
	}

	return msg, nil, nil
}

func (m *Manager) HasActiveDialog(ctx context.Context, userID model.GlobalUserID) bool {
	ds, err := m.storage.Load(ctx, userID)
	return err == nil && ds != nil
}

func (m *Manager) CancelCommand(ctx context.Context, userID model.GlobalUserID) error {
	return m.storage.Delete(ctx, userID)
}

func (m *Manager) RelocateDialog(ctx context.Context, userID model.GlobalUserID, chatID string) error {
	ds, err := m.storage.Load(ctx, userID)
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}
	if ds == nil {
		return nil
	}
	ds.ChatID = chatID
	return m.storage.Save(ctx, userID, *ds)
}

func (m *Manager) IsPreservesDialog(commandName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	def, ok := m.commands[commandName]
	if !ok {
		return false
	}
	return def.PreservesDialog
}

func (m *Manager) GetCurrentStepMessage(ctx context.Context, userID model.GlobalUserID, locale string) (*model.Message, string, error) {
	ds, err := m.storage.Load(ctx, userID)
	if err != nil {
		return nil, "", fmt.Errorf("loading state: %w", err)
	}
	if ds == nil {
		return nil, "", nil
	}

	m.mu.RLock()
	handler, ok := m.handlers[ds.CommandName]
	m.mu.RUnlock()
	if !ok {
		return nil, "", fmt.Errorf("%w: %s", ErrCommandNotFound, ds.CommandName)
	}

	state, err := handler.RestoreState(*ds)
	if err != nil {
		return nil, "", fmt.Errorf("restoring state: %w", err)
	}

	msg := handler.BuildStepMessage(state, locale)
	return &msg, ds.CommandName, nil
}
