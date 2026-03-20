package state

import "SuperBotGo/internal/model"

// StepContext provides contextual information to message builders when
// constructing step prompts. It carries the current user, locale,
// collected parameters, and current pagination page.
type StepContext struct {
	UserID model.GlobalUserID
	Locale string
	Params model.OptionMap
	Page   int
}
