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

func (a *StateManagerAdapter) StartCommand(ctx context.Context, userID model.GlobalUserID, _ model.ChannelType, chatID string, commandName string, locale string) (*StateResult, error) {

	if a.mgr.IsCommandImmediate(commandName) {

		if !a.mgr.IsPreservesDialog(commandName) {
			_ = a.mgr.CancelCommand(ctx, userID)
		}
		return &StateResult{
			Message:     model.Message{},
			CommandName: commandName,
			IsComplete:  true,
		}, nil
	}

	msg, err := a.mgr.StartCommand(ctx, userID, chatID, commandName, locale)
	if err != nil {
		return nil, err
	}
	return &StateResult{
		Message:     msg,
		CommandName: commandName,
		IsComplete:  false,
	}, nil
}

func (a *StateManagerAdapter) ProcessInput(ctx context.Context, userID model.GlobalUserID, _ model.ChannelType, chatID string, input model.UserInput, locale string) (*StateResult, error) {
	msg, cmdReq, err := a.mgr.ProcessInput(ctx, userID, chatID, input, locale)
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

func (a *StateManagerAdapter) IsPreservesDialog(commandName string) bool {
	return a.mgr.IsPreservesDialog(commandName)
}

func (a *StateManagerAdapter) GetCurrentStepMessage(ctx context.Context, userID model.GlobalUserID, locale string) (*model.Message, string, error) {
	return a.mgr.GetCurrentStepMessage(ctx, userID, locale)
}

func (a *StateManagerAdapter) RelocateDialog(ctx context.Context, userID model.GlobalUserID, chatID string) error {
	return a.mgr.RelocateDialog(ctx, userID, chatID)
}

var _ StateManager = (*StateManagerAdapter)(nil)
