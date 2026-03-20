package state

import "SuperBotGo/internal/model"

// StepOutcome holds the outcome of processing a single step: the message to
// display, the command name, completion status, and (when complete) the final
// collected parameters.
type StepOutcome struct {
	Message     model.Message
	CommandName string
	IsComplete  bool
	Params      model.OptionMap
}

// State represents the in-memory state of an active command dialog.
type State interface {
	IsComplete() bool
	FinalParams() model.OptionMap
}

// StateHandler defines the contract for managing command dialog lifecycles.
// Implementations handle creating, restoring, persisting, and advancing
// dialog state.
type StateHandler interface {
	// CreateNewState initializes a fresh state for the named command.
	CreateNewState(commandName string) (State, error)

	// RestoreState reconstructs an in-memory State from a persisted DialogState.
	RestoreState(ds model.DialogState) (State, error)

	// PersistState serializes the in-memory State into a DialogState for storage.
	PersistState(s State) model.DialogState

	// ProcessInput advances the state by processing the user's input.
	// It returns the updated state and the step outcome (message + completion info).
	ProcessInput(userID model.GlobalUserID, s State, input model.UserInput) (State, StepOutcome, error)

	// BuildStepMessage constructs the prompt message for the current step.
	BuildStepMessage(s State, locale string) model.Message
}
