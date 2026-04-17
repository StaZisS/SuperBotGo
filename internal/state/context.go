package state

import (
	"context"

	"SuperBotGo/internal/model"
)

type StepContext struct {
	Context context.Context
	UserID  model.GlobalUserID
	Locale  string
	Params  model.OptionMap
	Page    int
}
