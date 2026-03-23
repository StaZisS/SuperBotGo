package hostapi

import (
	"context"
	"testing"
)

func TestProviderRegisterAndGet(t *testing.T) {
	p := NewHostFunctionProvider()

	handler := func(ctx context.Context, pluginID string, request []byte) ([]byte, error) {
		return []byte("ok"), nil
	}

	p.Register(HostFunction{
		Name:        "test_func",
		Description: "A test function.",
		Permissions: []string{"test:read"},
		Handler:     handler,
	})

	fn, ok := p.Get("test_func")
	if !ok {
		t.Fatal("expected to find test_func")
	}
	if fn.Name != "test_func" {
		t.Errorf("expected name test_func, got %s", fn.Name)
	}
	if fn.Description != "A test function." {
		t.Errorf("unexpected description: %s", fn.Description)
	}
	if len(fn.Permissions) != 1 || fn.Permissions[0] != "test:read" {
		t.Errorf("unexpected permissions: %v", fn.Permissions)
	}

	// Handler should work.
	resp, err := fn.Handler(context.Background(), "p1", nil)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if string(resp) != "ok" {
		t.Errorf("expected 'ok', got %q", string(resp))
	}
}

func TestProviderGetMissing(t *testing.T) {
	p := NewHostFunctionProvider()
	_, ok := p.Get("nonexistent")
	if ok {
		t.Fatal("expected not found for nonexistent function")
	}
}

func TestProviderFunctionsSorted(t *testing.T) {
	p := NewHostFunctionProvider()
	noopH := func(ctx context.Context, pluginID string, request []byte) ([]byte, error) {
		return nil, nil
	}

	p.Register(HostFunction{Name: "zebra", Handler: noopH})
	p.Register(HostFunction{Name: "alpha", Handler: noopH})
	p.Register(HostFunction{Name: "middle", Handler: noopH})

	fns := p.Functions()
	if len(fns) != 3 {
		t.Fatalf("expected 3 functions, got %d", len(fns))
	}
	if fns[0].Name != "alpha" || fns[1].Name != "middle" || fns[2].Name != "zebra" {
		t.Errorf("functions not sorted: %s, %s, %s", fns[0].Name, fns[1].Name, fns[2].Name)
	}
}

func TestProviderLen(t *testing.T) {
	p := NewHostFunctionProvider()
	if p.Len() != 0 {
		t.Fatalf("expected 0, got %d", p.Len())
	}

	noopH := func(ctx context.Context, pluginID string, request []byte) ([]byte, error) {
		return nil, nil
	}
	p.Register(HostFunction{Name: "a", Handler: noopH})
	p.Register(HostFunction{Name: "b", Handler: noopH})

	if p.Len() != 2 {
		t.Fatalf("expected 2, got %d", p.Len())
	}
}

func TestProviderPanicsOnEmptyName(t *testing.T) {
	p := NewHostFunctionProvider()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for empty name")
		}
	}()
	p.Register(HostFunction{
		Name:    "",
		Handler: func(ctx context.Context, pluginID string, request []byte) ([]byte, error) { return nil, nil },
	})
}

func TestProviderPanicsOnNilHandler(t *testing.T) {
	p := NewHostFunctionProvider()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil handler")
		}
	}()
	p.Register(HostFunction{
		Name:    "test",
		Handler: nil,
	})
}

func TestProviderPanicsOnDuplicate(t *testing.T) {
	p := NewHostFunctionProvider()
	noopH := func(ctx context.Context, pluginID string, request []byte) ([]byte, error) {
		return nil, nil
	}
	p.Register(HostFunction{Name: "dup", Handler: noopH})

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for duplicate registration")
		}
	}()
	p.Register(HostFunction{Name: "dup", Handler: noopH})
}

func TestBuildProviderRegistersAllFunctions(t *testing.T) {
	h := NewHostAPI(Dependencies{})
	provider := h.buildProvider()

	expected := []string{
		"call_plugin",
		"db_query",
		"db_save",
		"http_request",
		"kv_delete",
		"kv_get",
		"kv_list",
		"kv_set",
		"publish_event",
	}

	if provider.Len() != len(expected) {
		t.Fatalf("expected %d functions, got %d", len(expected), provider.Len())
	}

	for _, name := range expected {
		if _, ok := provider.Get(name); !ok {
			t.Errorf("expected function %q to be registered", name)
		}
	}

	// Verify sorted order.
	fns := provider.Functions()
	for i, fn := range fns {
		if fn.Name != expected[i] {
			t.Errorf("position %d: expected %q, got %q", i, expected[i], fn.Name)
		}
	}
}
