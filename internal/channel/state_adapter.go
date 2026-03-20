package channel

import (
	"context"

	"SuperBotGo/internal/model"
	"SuperBotGo/internal/state"
)

type StateManagerAdapter struct {
	mgr *state.Manager
}

func NewStateManagerAdapter(mgr *state.Manager) *StateManagerAdapter {
	return &StateManagerAdapter{mgr: mgr}
}

func (a *StateManagerAdapter) Register(def *state.CommandDefinition) {
	a.mgr.RegisterCommand(def)
}

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

func (a *StateManagerAdapter) CancelCommand(ctx context.Context, userID model.GlobalUserID, _ model.ChannelType) error {
	return a.mgr.CancelCommand(ctx, userID)
}

var _ StateManager = (*StateManagerAdapter)(nil)
