package state

import "SuperBotGo/internal/model"

type StepOutcome struct {
	Message     model.Message
	CommandName string
	IsComplete  bool
	Params      model.OptionMap
}

type State interface {
	IsComplete() bool
	FinalParams() model.OptionMap
}

type StateHandler interface {
	CreateNewState(commandName string) (State, error)

	RestoreState(ds model.DialogState) (State, error)

	PersistState(s State) model.DialogState

	ProcessInput(userID model.GlobalUserID, s State, input model.UserInput) (State, StepOutcome, error)

	BuildStepMessage(s State, locale string) model.Message
}
