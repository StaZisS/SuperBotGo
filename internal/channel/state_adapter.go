package channel

import (
	"context"

	"SuperBotGo/internal/model"
	"SuperBotGo/internal/state"
)

// StateManagerAdapter wraps a state.Manager to implement the channel.StateManager
// interface. The channel-layer StateManager interface passes ChannelType for
// scoping, but the underlying state.Manager does not use it.
type StateManagerAdapter struct {
	mgr *state.Manager
}

// NewStateManagerAdapter creates an adapter around a state.Manager.
func NewStateManagerAdapter(mgr *state.Manager) *StateManagerAdapter {
	return &StateManagerAdapter{mgr: mgr}
}

// Register delegates to the underlying state.Manager.
func (a *StateManagerAdapter) Register(def *state.CommandDefinition) {
	a.mgr.RegisterCommand(def)
}

// StartCommand starts a new command dialog. If the command has no steps
// (e.g. Wasm plugins), it is immediately complete.
func (a *StateManagerAdapter) StartCommand(ctx context.Context, userID model.GlobalUserID, _ model.ChannelType, commandName string, locale string) (*StateResult, error) {

	if a.mgr.IsCommandImmediate(commandName) {

		_ = a.mgr.CancelCommand(ctx, userID)
		return &StateResult{
			Message:     model.Message{},
			CommandName: commandName,
			IsComplete:  true,
		}, nil
	}

	msg, err := a.mgr.StartCommand(ctx, userID, commandName, locale)
	if err != nil {
		return nil, err
	}
	return &StateResult{
		Message:     msg,
		CommandName: commandName,
		IsComplete:  false,
	}, nil
}

// ProcessInput processes user input within an active dialog.
func (a *StateManagerAdapter) ProcessInput(ctx context.Context, userID model.GlobalUserID, _ model.ChannelType, input model.UserInput, locale string) (*StateResult, error) {
	msg, cmdReq, err := a.mgr.ProcessInput(ctx, userID, input, locale)
	if err != nil {
		return nil, err
	}

	result := &StateResult{
		Message:    msg,
		IsComplete: cmdReq != nil,
	}

	if cmdReq != nil {
		result.CommandName = cmdReq.CommandName
		result.Params = cmdReq.Params
	}

	return result, nil
}

// CancelCommand cancels any active dialog for the user.
func (a *StateManagerAdapter) CancelCommand(ctx context.Context, userID model.GlobalUserID, _ model.ChannelType) error {
	return a.mgr.CancelCommand(ctx, userID)
}

// Ensure StateManagerAdapter implements StateManager.
var _ StateManager = (*StateManagerAdapter)(nil)
