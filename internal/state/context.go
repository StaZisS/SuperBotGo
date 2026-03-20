package state

import "SuperBotGo/internal/model"

type StepContext struct {
	UserID model.GlobalUserID
	Locale string
	Params model.OptionMap
	Page   int
}
