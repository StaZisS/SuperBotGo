package state

import (
	"testing"

	"SuperBotGo/internal/model"
)

// msgBuilder is a helper that returns a MessageBuilder producing a single TextBlock.
func msgBuilder(text string) func(StepContext) model.Message {
	return func(_ StepContext) model.Message {
		return model.NewTextMessage(text)
	}
}

// stepNode is a shorthand for creating a StepNode with a name and message text.
func stepNode(param, msg string) StepNode {
	return StepNode{
		ParamName:      param,
		MessageBuilder: msgBuilder(msg),
	}
}

func TestResolveActiveSteps_LinearSteps(t *testing.T) {
	cmd := &CommandDefinition{
		Name: "linear",
		Nodes: []CommandNode{
			stepNode("a", "step-a"),
			stepNode("b", "step-b"),
			stepNode("c", "step-c"),
		},
	}

	tests := []struct {
		name   string
		params model.OptionMap
		want   int
	}{
		{"empty params returns all steps", model.OptionMap{}, 3},
		{"partial params still returns all steps", model.OptionMap{"a": "1"}, 3},
		{"all filled still returns all steps", model.OptionMap{"a": "1", "b": "2", "c": "3"}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps := cmd.ResolveActiveSteps(tt.params)
			if len(steps) != tt.want {
				t.Fatalf("got %d steps, want %d", len(steps), tt.want)
			}
			// Verify order
			if steps[0].ParamName != "a" || steps[1].ParamName != "b" || steps[2].ParamName != "c" {
				t.Fatalf("unexpected step order: %s, %s, %s", steps[0].ParamName, steps[1].ParamName, steps[2].ParamName)
			}
		})
	}
}

func TestResolveActiveSteps_ConditionalStep(t *testing.T) {
	cmd := &CommandDefinition{
		Name: "cond",
		Nodes: []CommandNode{
			stepNode("a", "step-a"),
			StepNode{
				ParamName:      "b",
				MessageBuilder: msgBuilder("step-b"),
				Condition: func(params model.OptionMap) bool {
					return params["a"] == "yes"
				},
			},
			stepNode("c", "step-c"),
		},
	}

	tests := []struct {
		name       string
		params     model.OptionMap
		wantCount  int
		wantParams []string
	}{
		{
			"condition true includes step",
			model.OptionMap{"a": "yes"},
			3,
			[]string{"a", "b", "c"},
		},
		{
			"condition false skips step",
			model.OptionMap{"a": "no"},
			2,
			[]string{"a", "c"},
		},
		{
			"empty params condition false skips step",
			model.OptionMap{},
			2,
			[]string{"a", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps := cmd.ResolveActiveSteps(tt.params)
			if len(steps) != tt.wantCount {
				t.Fatalf("got %d steps, want %d", len(steps), tt.wantCount)
			}
			for i, wantParam := range tt.wantParams {
				if steps[i].ParamName != wantParam {
					t.Errorf("step[%d].ParamName = %q, want %q", i, steps[i].ParamName, wantParam)
				}
			}
		})
	}
}

func TestResolveActiveSteps_BranchNode(t *testing.T) {
	cmd := &CommandDefinition{
		Name: "branch",
		Nodes: []CommandNode{
			stepNode("type", "choose type"),
			BranchNode{
				OnParam: "type",
				Cases: map[string][]CommandNode{
					"text":  {stepNode("body", "enter text body")},
					"image": {stepNode("url", "enter image url")},
				},
				Default: []CommandNode{stepNode("fallback", "default step")},
			},
			stepNode("confirm", "confirm?"),
		},
	}

	tests := []struct {
		name       string
		params     model.OptionMap
		wantParams []string
	}{
		{
			"matches text case",
			model.OptionMap{"type": "text"},
			[]string{"type", "body", "confirm"},
		},
		{
			"matches image case",
			model.OptionMap{"type": "image"},
			[]string{"type", "url", "confirm"},
		},
		{
			"unknown value uses default",
			model.OptionMap{"type": "video"},
			[]string{"type", "fallback", "confirm"},
		},
		{
			"param not set uses default",
			model.OptionMap{},
			[]string{"type", "fallback", "confirm"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps := cmd.ResolveActiveSteps(tt.params)
			if len(steps) != len(tt.wantParams) {
				t.Fatalf("got %d steps, want %d", len(steps), len(tt.wantParams))
			}
			for i, wantParam := range tt.wantParams {
				if steps[i].ParamName != wantParam {
					t.Errorf("step[%d].ParamName = %q, want %q", i, steps[i].ParamName, wantParam)
				}
			}
		})
	}
}

func TestResolveActiveSteps_ConditionalBranchNode(t *testing.T) {
	cmd := &CommandDefinition{
		Name: "condbranch",
		Nodes: []CommandNode{
			stepNode("role", "your role"),
			ConditionalBranchNode{
				Cases: []ConditionalCase{
					{
						Predicate: func(p model.OptionMap) bool { return p["role"] == "admin" },
						Nodes:     []CommandNode{stepNode("secret", "admin secret")},
					},
					{
						Predicate: func(p model.OptionMap) bool { return p["role"] == "mod" },
						Nodes:     []CommandNode{stepNode("channel", "mod channel")},
					},
				},
				Default: []CommandNode{stepNode("name", "enter your name")},
			},
		},
	}

	tests := []struct {
		name       string
		params     model.OptionMap
		wantParams []string
	}{
		{
			"first case matches",
			model.OptionMap{"role": "admin"},
			[]string{"role", "secret"},
		},
		{
			"second case matches",
			model.OptionMap{"role": "mod"},
			[]string{"role", "channel"},
		},
		{
			"no case matches uses default",
			model.OptionMap{"role": "user"},
			[]string{"role", "name"},
		},
		{
			"empty params uses default",
			model.OptionMap{},
			[]string{"role", "name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps := cmd.ResolveActiveSteps(tt.params)
			if len(steps) != len(tt.wantParams) {
				t.Fatalf("got %d steps, want %d", len(steps), len(tt.wantParams))
			}
			for i, wantParam := range tt.wantParams {
				if steps[i].ParamName != wantParam {
					t.Errorf("step[%d].ParamName = %q, want %q", i, steps[i].ParamName, wantParam)
				}
			}
		})
	}
}

func TestResolveActiveSteps_NestedBranches(t *testing.T) {
	cmd := &CommandDefinition{
		Name: "nested",
		Nodes: []CommandNode{
			stepNode("level1", "choose level1"),
			BranchNode{
				OnParam: "level1",
				Cases: map[string][]CommandNode{
					"a": {
						stepNode("level2", "choose level2"),
						BranchNode{
							OnParam: "level2",
							Cases: map[string][]CommandNode{
								"x": {stepNode("deep_x", "deep x step")},
								"y": {stepNode("deep_y", "deep y step")},
							},
							Default: []CommandNode{stepNode("deep_default", "deep default")},
						},
					},
				},
				Default: []CommandNode{stepNode("other", "other path")},
			},
		},
	}

	tests := []struct {
		name       string
		params     model.OptionMap
		wantParams []string
	}{
		{
			"outer a, inner x",
			model.OptionMap{"level1": "a", "level2": "x"},
			[]string{"level1", "level2", "deep_x"},
		},
		{
			"outer a, inner y",
			model.OptionMap{"level1": "a", "level2": "y"},
			[]string{"level1", "level2", "deep_y"},
		},
		{
			"outer a, inner unset uses inner default",
			model.OptionMap{"level1": "a"},
			[]string{"level1", "level2", "deep_default"},
		},
		{
			"outer unset uses outer default",
			model.OptionMap{},
			[]string{"level1", "other"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps := cmd.ResolveActiveSteps(tt.params)
			if len(steps) != len(tt.wantParams) {
				t.Fatalf("got %d steps, want %d", len(steps), len(tt.wantParams))
			}
			for i, wantParam := range tt.wantParams {
				if steps[i].ParamName != wantParam {
					t.Errorf("step[%d].ParamName = %q, want %q", i, steps[i].ParamName, wantParam)
				}
			}
		})
	}
}

func TestCurrentStep_ReturnsFirstUnfilled(t *testing.T) {
	cmd := &CommandDefinition{
		Name: "current",
		Nodes: []CommandNode{
			stepNode("a", "step-a"),
			stepNode("b", "step-b"),
			stepNode("c", "step-c"),
		},
	}

	tests := []struct {
		name      string
		params    model.OptionMap
		wantParam string
	}{
		{"no params filled", model.OptionMap{}, "a"},
		{"first filled", model.OptionMap{"a": "1"}, "b"},
		{"first two filled", model.OptionMap{"a": "1", "b": "2"}, "c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := cmd.CurrentStep(tt.params)
			if step == nil {
				t.Fatal("expected a step, got nil")
			}
			if step.ParamName != tt.wantParam {
				t.Errorf("CurrentStep().ParamName = %q, want %q", step.ParamName, tt.wantParam)
			}
		})
	}
}

func TestCurrentStep_NilWhenComplete(t *testing.T) {
	cmd := &CommandDefinition{
		Name: "done",
		Nodes: []CommandNode{
			stepNode("a", "step-a"),
			stepNode("b", "step-b"),
		},
	}

	params := model.OptionMap{"a": "1", "b": "2"}
	step := cmd.CurrentStep(params)
	if step != nil {
		t.Errorf("expected nil when all params filled, got step %q", step.ParamName)
	}
}

func TestIsComplete(t *testing.T) {
	cmd := &CommandDefinition{
		Name: "complete",
		Nodes: []CommandNode{
			stepNode("a", "step-a"),
			stepNode("b", "step-b"),
		},
	}

	tests := []struct {
		name     string
		params   model.OptionMap
		wantDone bool
	}{
		{"nil params acts as empty - not complete", model.OptionMap{}, false},
		{"partial params not complete", model.OptionMap{"a": "1"}, false},
		{"all params filled is complete", model.OptionMap{"a": "1", "b": "2"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cmd.IsComplete(tt.params)
			if got != tt.wantDone {
				t.Errorf("IsComplete() = %v, want %v", got, tt.wantDone)
			}
		})
	}
}

func TestIsComplete_NoSteps(t *testing.T) {
	cmd := &CommandDefinition{
		Name:  "empty",
		Nodes: nil,
	}

	// A command with no steps is always complete.
	if !cmd.IsComplete(model.OptionMap{}) {
		t.Error("expected IsComplete=true for command with no steps")
	}
}
