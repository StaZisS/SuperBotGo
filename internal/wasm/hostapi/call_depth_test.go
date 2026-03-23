package hostapi

import (
	"context"
	"testing"
)

func TestCallDepthFromContext_Default(t *testing.T) {
	ctx := context.Background()
	if got := callDepthFromContext(ctx); got != 0 {
		t.Errorf("expected depth 0 for empty context, got %d", got)
	}
}

func TestCallChainFromContext_Default(t *testing.T) {
	ctx := context.Background()
	if got := callChainFromContext(ctx); got != nil {
		t.Errorf("expected nil chain for empty context, got %v", got)
	}
}

func TestWithCallDepth_IncrementsDepth(t *testing.T) {
	ctx := context.Background()

	ctx = withCallDepth(ctx, "pluginA")
	if got := callDepthFromContext(ctx); got != 1 {
		t.Errorf("expected depth 1, got %d", got)
	}

	ctx = withCallDepth(ctx, "pluginB")
	if got := callDepthFromContext(ctx); got != 2 {
		t.Errorf("expected depth 2, got %d", got)
	}

	ctx = withCallDepth(ctx, "pluginC")
	if got := callDepthFromContext(ctx); got != 3 {
		t.Errorf("expected depth 3, got %d", got)
	}
}

func TestWithCallDepth_BuildsChain(t *testing.T) {
	ctx := context.Background()
	ctx = withCallDepth(ctx, "A")
	ctx = withCallDepth(ctx, "B")
	ctx = withCallDepth(ctx, "C")

	chain := callChainFromContext(ctx)
	expected := []string{"A", "B", "C"}

	if len(chain) != len(expected) {
		t.Fatalf("expected chain length %d, got %d: %v", len(expected), len(chain), chain)
	}
	for i, id := range expected {
		if chain[i] != id {
			t.Errorf("chain[%d]: expected %q, got %q", i, id, chain[i])
		}
	}
}

func TestWithCallDepth_DoesNotMutatePreviousContext(t *testing.T) {
	ctx := context.Background()
	ctx1 := withCallDepth(ctx, "A")
	ctx2 := withCallDepth(ctx1, "B")

	// ctx1 chain should still be just ["A"]
	chain1 := callChainFromContext(ctx1)
	if len(chain1) != 1 || chain1[0] != "A" {
		t.Errorf("ctx1 chain mutated: %v", chain1)
	}

	chain2 := callChainFromContext(ctx2)
	if len(chain2) != 2 {
		t.Errorf("ctx2 chain length expected 2, got %d: %v", len(chain2), chain2)
	}
}

func TestCheckCallCycle_NoCycle(t *testing.T) {
	chain := []string{"A", "B", "C"}
	if err := checkCallCycle(chain, "D"); err != nil {
		t.Errorf("expected no cycle, got: %v", err)
	}
}

func TestCheckCallCycle_DirectCycle(t *testing.T) {
	chain := []string{"A", "B"}
	err := checkCallCycle(chain, "A")
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	expected := "circular inter-plugin call detected: A -> B -> A"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

func TestCheckCallCycle_IndirectCycle(t *testing.T) {
	chain := []string{"A", "B", "C"}
	err := checkCallCycle(chain, "B")
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	expected := "circular inter-plugin call detected: A -> B -> C -> B"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

func TestCheckCallCycle_SelfCall(t *testing.T) {
	chain := []string{"A"}
	err := checkCallCycle(chain, "A")
	if err == nil {
		t.Fatal("expected cycle error for self-call, got nil")
	}
	expected := "circular inter-plugin call detected: A -> A"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

func TestCheckCallCycle_EmptyChain(t *testing.T) {
	if err := checkCallCycle(nil, "A"); err != nil {
		t.Errorf("expected no cycle for empty chain, got: %v", err)
	}
	if err := checkCallCycle([]string{}, "A"); err != nil {
		t.Errorf("expected no cycle for empty chain, got: %v", err)
	}
}

func TestMaxCallDepth_Value(t *testing.T) {
	if MaxCallDepth != 5 {
		t.Errorf("expected MaxCallDepth = 5, got %d", MaxCallDepth)
	}
}
