package state

import (
	"errors"
	"fmt"

	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/model"
)

const (
	PageNext = "__page_next"
	PagePrev = "__page_prev"
)

type DslState struct {
	Command   *CommandDefinition
	Params    model.OptionMap
	PageState map[string]int
}

func (s *DslState) IsComplete() bool {
	return s.Command.IsComplete(s.Params)
}

func (s *DslState) FinalParams() model.OptionMap {
	result := make(model.OptionMap, len(s.Params))
	for k, v := range s.Params {
		result[k] = v
	}
	return result
}

type DslStateHandler struct {
	command *CommandDefinition
}

func NewDslStateHandler(command *CommandDefinition) *DslStateHandler {
	return &DslStateHandler{command: command}
}

func (h *DslStateHandler) CreateNewState(_ string) (State, error) {
	return &DslState{
		Command:   h.command,
		Params:    make(model.OptionMap),
		PageState: make(map[string]int),
	}, nil
}

func (h *DslStateHandler) RestoreState(ds model.DialogState) (State, error) {
	params := make(model.OptionMap, len(ds.Params))
	for k, v := range ds.Params {
		params[k] = v
	}
	pageState := make(map[string]int, len(ds.PageState))
	for k, v := range ds.PageState {
		pageState[k] = v
	}
	return &DslState{
		Command:   h.command,
		Params:    params,
		PageState: pageState,
	}, nil
}

func (h *DslStateHandler) PersistState(s State) model.DialogState {
	ds := requireDslState(s)
	params := make(model.OptionMap, len(ds.Params))
	for k, v := range ds.Params {
		params[k] = v
	}
	pageState := make(map[string]int, len(ds.PageState))
	for k, v := range ds.PageState {
		pageState[k] = v
	}
	return model.DialogState{
		CommandName: h.command.Name,
		Params:      params,
		PageState:   pageState,
	}
}

func (h *DslStateHandler) ProcessInput(_ model.GlobalUserID, s State, input model.UserInput) (State, StepOutcome, error) {
	ds := requireDslState(s)
	step := h.command.CurrentStep(ds.Params)
	if step == nil {

		return ds, StepOutcome{
			CommandName: h.command.Name,
			IsComplete:  true,
			Params:      ds.FinalParams(),
		}, nil
	}

	if step.Pagination != nil {
		if _, isCallback := input.(model.CallbackInput); isCallback {
			textVal := input.TextValue()
			switch textVal {
			case PageNext:
				cur := ds.PageState[step.ParamName]
				ds.PageState[step.ParamName] = cur + 1
				msg := h.BuildStepMessage(ds, "")
				return ds, StepOutcome{
					Message:     msg,
					CommandName: h.command.Name,
					IsComplete:  false,
				}, nil
			case PagePrev:
				cur := ds.PageState[step.ParamName]
				if cur > 0 {
					ds.PageState[step.ParamName] = cur - 1
				}
				msg := h.BuildStepMessage(ds, "")
				return ds, StepOutcome{
					Message:     msg,
					CommandName: h.command.Name,
					IsComplete:  false,
				}, nil
			}
		}
	}

	isValid := true
	if step.Validate != nil {
		isValid = step.Validate(input)
	}

	if isValid {
		ds.Params[step.ParamName] = input.TextValue()
	}

	msg := h.BuildStepMessage(ds, "")
	complete := ds.IsComplete()

	outcome := StepOutcome{
		Message:     msg,
		CommandName: h.command.Name,
		IsComplete:  complete,
	}
	if complete {
		outcome.Params = ds.FinalParams()
	}

	return ds, outcome, nil
}

func (h *DslStateHandler) BuildStepMessage(s State, locale string) model.Message {
	ds := requireDslState(s)
	step := h.command.CurrentStep(ds.Params)
	if step == nil {
		return model.Message{
			Blocks: []model.ContentBlock{
				model.TextBlock{Text: i18n.Get("common.command_completed", locale), Style: model.StylePlain},
			},
		}
	}

	ctx := StepContext{
		Params: ds.Params,
		Locale: locale,
	}

	message := step.MessageBuilder(ctx)

	if step.Pagination != nil {
		return h.applyPagination(message, step, ds, locale)
	}

	return message
}

func (h *DslStateHandler) applyPagination(message model.Message, step *StepNode, ds *DslState, locale string) model.Message {
	config := step.Pagination
	currentPage := ds.PageState[step.ParamName]
	ctx := StepContext{
		Params: ds.Params,
		Locale: locale,
		Page:   currentPage,
	}
	result := config.PageProvider(ctx, currentPage)

	var navOptions []model.Option
	if currentPage > 0 {
		navOptions = append(navOptions, model.Option{Label: "Previous", Value: PagePrev})
	}
	if result.HasMore {
		navOptions = append(navOptions, model.Option{Label: "Next", Value: PageNext})
	}

	allOptions := make([]model.Option, 0, len(result.Options)+len(navOptions))
	allOptions = append(allOptions, result.Options...)
	allOptions = append(allOptions, navOptions...)

	paginatedBlock := model.OptionsBlock{
		Prompt:  config.Prompt,
		Options: allOptions,
	}

	blocks := make([]model.ContentBlock, 0, len(message.Blocks)+1)
	blocks = append(blocks, message.Blocks...)
	blocks = append(blocks, paginatedBlock)

	return model.Message{Blocks: blocks}
}

func requireDslState(s State) *DslState {
	ds, ok := s.(*DslState)
	if !ok {
		panic(fmt.Sprintf("expected *DslState but got %T", s))
	}
	return ds
}

var _ StateHandler = (*DslStateHandler)(nil)

var ErrCommandNotFound = errors.New("command not found")

var ErrNoActiveDialog = errors.New("no active dialog")
