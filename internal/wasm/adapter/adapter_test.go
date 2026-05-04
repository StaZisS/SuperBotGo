package adapter

import (
	"encoding/json"
	"reflect"
	"testing"

	"SuperBotGo/internal/model"
	"SuperBotGo/internal/state"
	wasmrt "SuperBotGo/internal/wasm/runtime"
)

func TestBranchNodeRoundTrip(t *testing.T) {
	nodesJSON := `[
		{"type":"step","param":"mode","blocks":[{"type":"options","prompt":"Choose:","options":[{"label":"Quick","value":"quick"},{"label":"By date","value":"by_date"}]}]},
		{"type":"step","param":"building","blocks":[{"type":"text","text":"Building:","style":"plain"}]},
		{"type":"step","param":"room","blocks":[{"type":"text","text":"Room:","style":"plain"}]},
		{"type":"branch","on_param":"mode","cases":{"by_date":[{"type":"step","param":"date","blocks":[{"type":"text","text":"Enter date:","style":"plain"}],"validation":"^\\d{4}-\\d{2}-\\d{2}$"}]}}
	]`

	var nodeDefs []wasmrt.NodeDef
	if err := json.Unmarshal([]byte(nodesJSON), &nodeDefs); err != nil {
		t.Fatalf("unmarshal NodeDefs: %v", err)
	}

	if len(nodeDefs) != 4 {
		t.Fatalf("expected 4 NodeDefs, got %d", len(nodeDefs))
	}

	branch := nodeDefs[3]
	t.Logf("branch NodeDef: type=%q on_param=%q cases=%v", branch.Type, branch.OnParam, branch.Cases)
	if branch.Type != "branch" {
		t.Fatalf("expected type=branch, got %q", branch.Type)
	}
	if branch.OnParam != "mode" {
		t.Fatalf("expected on_param=mode, got %q", branch.OnParam)
	}
	byDate, ok := branch.Cases["by_date"]
	if !ok {
		t.Fatalf("missing by_date case in Cases map (keys: %v)", keysOf(branch.Cases))
	}
	if len(byDate) == 0 {
		t.Fatalf("by_date case is empty")
	}
	if byDate[0].Param != "date" {
		t.Fatalf("expected date step, got param=%q", byDate[0].Param)
	}

	wp := &WasmPlugin{}

	var commandNodes []state.CommandNode
	for _, nd := range nodeDefs {
		if cn := wp.nodeDefToCommandNode(nd); cn != nil {
			commandNodes = append(commandNodes, cn)
		}
	}

	if len(commandNodes) != 4 {
		t.Fatalf("expected 4 CommandNodes, got %d", len(commandNodes))
	}

	if _, ok := commandNodes[0].(state.StepNode); !ok {
		t.Fatalf("node[0]: expected StepNode, got %T", commandNodes[0])
	}
	if _, ok := commandNodes[3].(state.BranchNode); !ok {
		t.Fatalf("node[3]: expected BranchNode, got %T", commandNodes[3])
	}

	bn := commandNodes[3].(state.BranchNode)
	if bn.OnParam != "mode" {
		t.Fatalf("BranchNode.OnParam = %q, want %q", bn.OnParam, "mode")
	}
	if _, ok := bn.Cases["by_date"]; !ok {
		t.Fatalf("BranchNode missing by_date case")
	}

	cmdDef := &state.CommandDefinition{
		Name:  "schedule",
		Nodes: commandNodes,
	}

	quickParams := model.OptionMap{"mode": "quick", "building": "1", "room": "101"}
	quickSteps := cmdDef.ResolveActiveSteps(state.StepContext{Params: quickParams})
	quickNames := stepNames(quickSteps)
	t.Logf("quick path: %v", quickNames)

	if contains(quickNames, "date") {
		t.Fatalf("quick path should NOT contain date step, got %v", quickNames)
	}
	if cmdDef.CurrentStep(state.StepContext{Params: quickParams}) != nil {
		t.Fatalf("quick path should be complete")
	}

	byDateParams := model.OptionMap{"mode": "by_date", "building": "1", "room": "101"}
	byDateSteps := cmdDef.ResolveActiveSteps(state.StepContext{Params: byDateParams})
	byDateNames := stepNames(byDateSteps)
	t.Logf("by_date path: %v", byDateNames)

	if !contains(byDateNames, "date") {
		t.Fatalf("by_date path MUST contain date step, got %v", byDateNames)
	}

	cur := cmdDef.CurrentStep(state.StepContext{Params: byDateParams})
	if cur == nil {
		t.Fatalf("by_date path should NOT be complete (date not filled)")
	}
	if cur.ParamName != "date" {
		t.Fatalf("expected CurrentStep=date, got %q", cur.ParamName)
	}

	fullParams := model.OptionMap{"mode": "by_date", "building": "1", "room": "101", "date": "2026-03-25"}
	if !cmdDef.IsComplete(state.StepContext{Params: fullParams}) {
		t.Fatalf("by_date path with date filled should be complete")
	}
}

func TestCommandsCarriesLocalizedDescriptions(t *testing.T) {
	wp := &WasmPlugin{
		meta: wasmrt.PluginMeta{
			Triggers: []wasmrt.TriggerDef{
				{
					Name: "hello",
					Type: "messenger",
					Descriptions: map[string]string{
						"en": "Say hello",
						"ru": "Поздороваться",
					},
					Description: "Say hello",
				},
				{Name: "hook", Type: "http", Description: "Webhook"},
			},
		},
	}

	commands := wp.Commands()
	if len(commands) != 1 {
		t.Fatalf("Commands() len = %d, want 1", len(commands))
	}
	if commands[0].Description != "Say hello" {
		t.Errorf("Description = %q, want %q", commands[0].Description, "Say hello")
	}
	want := map[string]string{"en": "Say hello", "ru": "Поздороваться"}
	if !reflect.DeepEqual(commands[0].Descriptions, want) {
		t.Errorf("Descriptions = %#v, want %#v", commands[0].Descriptions, want)
	}
}

func stepNames(steps []state.StepNode) []string {
	names := make([]string, len(steps))
	for i, s := range steps {
		names[i] = s.ParamName
	}
	return names
}

func contains(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}

func keysOf(m map[string][]wasmrt.NodeDef) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
