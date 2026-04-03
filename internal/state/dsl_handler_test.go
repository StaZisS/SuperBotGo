package state

import (
	"testing"

	"SuperBotGo/internal/model"
)

// newTestCommand creates a simple two-step command for handler tests.
func newTestCommand() *CommandDefinition {
	return &CommandDefinition{
		Name:        "test_cmd",
		Description: "test command",
		Nodes: []CommandNode{
			StepNode{
				ParamName:      "name",
				MessageBuilder: msgBuilder("enter name"),
				Validate: func(input model.UserInput) bool {
					return input.TextValue() != ""
				},
			},
			StepNode{
				ParamName:      "age",
				MessageBuilder: msgBuilder("enter age"),
			},
		},
	}
}

func TestCreateNewState(t *testing.T) {
	handler := NewDslStateHandler(newTestCommand())

	s, err := handler.CreateNewState("test_cmd")
	if err != nil {
		t.Fatalf("CreateNewState() error = %v", err)
	}

	ds, ok := s.(*DslState)
	if !ok {
		t.Fatalf("expected *DslState, got %T", s)
	}

	if ds.Command.Name != "test_cmd" {
		t.Errorf("Command.Name = %q, want %q", ds.Command.Name, "test_cmd")
	}
	if len(ds.Params) != 0 {
		t.Errorf("Params should be empty, got %v", ds.Params)
	}
	if len(ds.PageState) != 0 {
		t.Errorf("PageState should be empty, got %v", ds.PageState)
	}
	if s.IsComplete() {
		t.Error("new state should not be complete")
	}
}

func TestProcessInput_CapturesParam(t *testing.T) {
	handler := NewDslStateHandler(newTestCommand())
	s, _ := handler.CreateNewState("test_cmd")

	input := model.TextInput{Text: "Alice"}
	newState, outcome, err := handler.ProcessInput(0, s, input)
	if err != nil {
		t.Fatalf("ProcessInput() error = %v", err)
	}

	ds := newState.(*DslState)
	if ds.Params["name"] != "Alice" {
		t.Errorf("Params[name] = %q, want %q", ds.Params["name"], "Alice")
	}

	if outcome.IsComplete {
		t.Error("should not be complete after first step")
	}
	if outcome.CommandName != "test_cmd" {
		t.Errorf("outcome.CommandName = %q, want %q", outcome.CommandName, "test_cmd")
	}

	// The message should be for the next step (age).
	if outcome.Message.IsEmpty() {
		t.Error("expected a non-empty message for the next step")
	}
}

func TestProcessInput_ValidationFails(t *testing.T) {
	handler := NewDslStateHandler(newTestCommand())
	s, _ := handler.CreateNewState("test_cmd")

	// Empty text should fail validation for "name" step.
	input := model.TextInput{Text: ""}
	newState, outcome, err := handler.ProcessInput(0, s, input)
	if err != nil {
		t.Fatalf("ProcessInput() error = %v", err)
	}

	ds := newState.(*DslState)
	if _, exists := ds.Params["name"]; exists {
		t.Error("param 'name' should not be set after validation failure")
	}
	if outcome.IsComplete {
		t.Error("should not be complete after validation failure")
	}
}

func TestProcessInput_CompletesCommand(t *testing.T) {
	handler := NewDslStateHandler(newTestCommand())
	s, _ := handler.CreateNewState("test_cmd")

	// Fill first step.
	s, _, _ = handler.ProcessInput(0, s, model.TextInput{Text: "Alice"})

	// Fill second step.
	newState, outcome, err := handler.ProcessInput(0, s, model.TextInput{Text: "30"})
	if err != nil {
		t.Fatalf("ProcessInput() error = %v", err)
	}

	if !outcome.IsComplete {
		t.Error("expected IsComplete=true after filling all steps")
	}
	if outcome.Params["name"] != "Alice" {
		t.Errorf("outcome.Params[name] = %q, want %q", outcome.Params["name"], "Alice")
	}
	if outcome.Params["age"] != "30" {
		t.Errorf("outcome.Params[age] = %q, want %q", outcome.Params["age"], "30")
	}
	if !newState.IsComplete() {
		t.Error("state.IsComplete() should be true")
	}
}

func TestProcessInput_Pagination(t *testing.T) {
	pageData := map[int]OptionsPage{
		0: {
			Options: []model.Option{{Label: "Item1", Value: "1"}},
			HasMore: true,
		},
		1: {
			Options: []model.Option{{Label: "Item2", Value: "2"}},
			HasMore: false,
		},
	}

	cmd := &CommandDefinition{
		Name: "paginated_cmd",
		Nodes: []CommandNode{
			StepNode{
				ParamName:      "item",
				MessageBuilder: msgBuilder("choose item"),
				Pagination: &PaginationConfig{
					Prompt:   "Pick one",
					PageSize: 1,
					PageProvider: func(_ StepContext, page int) OptionsPage {
						if p, ok := pageData[page]; ok {
							return p
						}
						return OptionsPage{}
					},
				},
			},
		},
	}

	handler := NewDslStateHandler(cmd)
	s, _ := handler.CreateNewState("paginated_cmd")

	// Navigate to next page.
	nextInput := model.CallbackInput{Data: PageNext}
	s, outcome, err := handler.ProcessInput(0, s, nextInput)
	if err != nil {
		t.Fatalf("ProcessInput(PageNext) error = %v", err)
	}
	if outcome.IsComplete {
		t.Error("should not be complete after pagination")
	}

	ds := s.(*DslState)
	if ds.PageState["item"] != 1 {
		t.Errorf("PageState[item] = %d, want 1", ds.PageState["item"])
	}

	// Navigate back to previous page.
	prevInput := model.CallbackInput{Data: PagePrev}
	s, outcome, err = handler.ProcessInput(0, s, prevInput)
	if err != nil {
		t.Fatalf("ProcessInput(PagePrev) error = %v", err)
	}
	if outcome.IsComplete {
		t.Error("should not be complete after pagination")
	}

	ds = s.(*DslState)
	if ds.PageState["item"] != 0 {
		t.Errorf("PageState[item] = %d, want 0 after prev", ds.PageState["item"])
	}

	// PagePrev at page 0 stays at 0.
	s, _, _ = handler.ProcessInput(0, s, prevInput)
	ds = s.(*DslState)
	if ds.PageState["item"] != 0 {
		t.Errorf("PageState[item] = %d, want 0 (should not go negative)", ds.PageState["item"])
	}
}

func TestPersistAndRestore(t *testing.T) {
	handler := NewDslStateHandler(newTestCommand())
	s, _ := handler.CreateNewState("test_cmd")

	// Fill one param and set page state.
	s, _, _ = handler.ProcessInput(0, s, model.TextInput{Text: "Bob"})
	ds := s.(*DslState)
	ds.PageState["some_key"] = 3

	// Persist.
	persisted := handler.PersistState(s)

	if persisted.CommandName != "test_cmd" {
		t.Errorf("persisted.CommandName = %q, want %q", persisted.CommandName, "test_cmd")
	}
	if persisted.Params["name"] != "Bob" {
		t.Errorf("persisted.Params[name] = %q, want %q", persisted.Params["name"], "Bob")
	}
	if persisted.PageState["some_key"] != 3 {
		t.Errorf("persisted.PageState[some_key] = %d, want 3", persisted.PageState["some_key"])
	}

	// Restore.
	restored, err := handler.RestoreState(persisted)
	if err != nil {
		t.Fatalf("RestoreState() error = %v", err)
	}

	rds := restored.(*DslState)
	if rds.Params["name"] != "Bob" {
		t.Errorf("restored Params[name] = %q, want %q", rds.Params["name"], "Bob")
	}
	if rds.PageState["some_key"] != 3 {
		t.Errorf("restored PageState[some_key] = %d, want 3", rds.PageState["some_key"])
	}

	// Verify isolation: mutating restored state does not affect persisted data.
	rds.Params["name"] = "CHANGED"
	if persisted.Params["name"] != "Bob" {
		t.Error("mutating restored state affected persisted data (missing copy)")
	}
}

func TestBuildStepMessage(t *testing.T) {
	cmd := &CommandDefinition{
		Name: "msg_cmd",
		Nodes: []CommandNode{
			StepNode{
				ParamName:      "color",
				MessageBuilder: msgBuilder("pick a color"),
			},
			StepNode{
				ParamName:      "size",
				MessageBuilder: msgBuilder("pick a size"),
			},
		},
	}

	handler := NewDslStateHandler(cmd)

	tests := []struct {
		name     string
		params   model.OptionMap
		wantText string
		wantNil  bool
	}{
		{
			"first step message",
			model.OptionMap{},
			"pick a color",
			false,
		},
		{
			"second step message",
			model.OptionMap{"color": "red"},
			"pick a size",
			false,
		},
		{
			"all filled returns empty message",
			model.OptionMap{"color": "red", "size": "large"},
			"",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &DslState{
				Command:   cmd,
				Params:    tt.params,
				PageState: make(map[string]int),
			}

			msg := handler.BuildStepMessage(s, "en")

			if tt.wantNil {
				if !msg.IsEmpty() {
					t.Errorf("expected empty message when complete, got %d blocks", len(msg.Blocks))
				}
				return
			}

			if msg.IsEmpty() {
				t.Fatal("expected non-empty message")
			}

			tb, ok := msg.Blocks[0].(model.TextBlock)
			if !ok {
				t.Fatalf("expected TextBlock, got %T", msg.Blocks[0])
			}
			if tb.Text != tt.wantText {
				t.Errorf("message text = %q, want %q", tb.Text, tt.wantText)
			}
		})
	}
}

func TestBuildStepMessage_WithLocale(t *testing.T) {
	cmd := &CommandDefinition{
		Name: "locale_cmd",
		Nodes: []CommandNode{
			StepNode{
				ParamName: "q",
				MessageBuilder: func(ctx StepContext) model.Message {
					return model.NewTextMessage("locale=" + ctx.Locale)
				},
			},
		},
	}

	handler := NewDslStateHandler(cmd)
	s := &DslState{
		Command:   cmd,
		Params:    model.OptionMap{},
		PageState: make(map[string]int),
	}

	msg := handler.BuildStepMessage(s, "ru")
	tb := msg.Blocks[0].(model.TextBlock)
	if tb.Text != "locale=ru" {
		t.Errorf("expected locale=ru, got %q", tb.Text)
	}
}
