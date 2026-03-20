package wasmplugin

import (
	"encoding/json"
	"testing"
)

func TestBranchSerialization(t *testing.T) {
	cmd := Command{
		Name: "schedule",
		Nodes: []Node{
			NewStep("mode").
				Options("Choose:",
					Opt("Quick", "quick"),
					Opt("By date", "by_date"),
				),
			NewStep("building").Text("Building:", StylePlain),
			NewStep("room").Text("Room:", StylePlain),
			BranchOn("mode",
				Case("by_date",
					NewStep("date").Text("Enter date:", StylePlain).Validate(`^\d{4}-\d{2}-\d{2}$`),
				),
			),
		},
	}

	// Serialize (as handleMeta does).
	reg := make(callbackMap)
	var nodes []nodeDef
	for _, node := range cmd.Nodes {
		nodes = append(nodes, node.toNodeDef(cmd.Name, reg))
	}

	if len(nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(nodes))
	}

	// Verify branch node structure before JSON.
	branch := nodes[3]
	if branch.Type != "branch" {
		t.Fatalf("node[3]: expected type=branch, got %q", branch.Type)
	}
	if branch.OnParam != "mode" {
		t.Fatalf("node[3]: expected on_param=mode, got %q", branch.OnParam)
	}
	byDate, ok := branch.Cases["by_date"]
	if !ok || len(byDate) == 0 {
		t.Fatalf("node[3]: missing by_date case, cases=%v", branch.Cases)
	}
	if byDate[0].Type != "step" || byDate[0].Param != "date" {
		t.Fatalf("node[3].cases.by_date[0]: expected step/date, got %s/%s", byDate[0].Type, byDate[0].Param)
	}

	// JSON round-trip (SDK → JSON → SDK types, simulating host).
	data, err := json.Marshal(nodes)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	t.Logf("JSON:\n%s", string(data))

	var parsed []nodeDef
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(parsed) != 4 {
		t.Fatalf("round-trip: expected 4 nodes, got %d", len(parsed))
	}
	if parsed[3].Type != "branch" {
		t.Fatalf("round-trip: node[3] type=%q", parsed[3].Type)
	}
	if parsed[3].OnParam != "mode" {
		t.Fatalf("round-trip: on_param=%q", parsed[3].OnParam)
	}
	rt, ok := parsed[3].Cases["by_date"]
	if !ok || len(rt) == 0 {
		t.Fatalf("round-trip: missing by_date case")
	}
	if rt[0].Param != "date" {
		t.Fatalf("round-trip: expected param=date, got %q", rt[0].Param)
	}
}
