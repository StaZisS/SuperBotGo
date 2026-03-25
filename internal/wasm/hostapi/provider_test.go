package hostapi

import (
	"context"
	"testing"
)

func mustRegister(t *testing.T, p *HostFunctionProvider, fn HostFunction) {
	t.Helper()
	if err := p.Register(fn); err != nil {
		t.Fatalf("Register(%q): %v", fn.Name, err)
	}
}

func TestProviderRegisterAndGet(t *testing.T) {
	p := NewHostFunctionProvider()

	handler := func(ctx context.Context, pluginID string, request []byte) ([]byte, error) {
		return []byte("ok"), nil
	}

	mustRegister(t, p, HostFunction{
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

	mustRegister(t, p, HostFunction{Name: "zebra", Handler: noopH})
	mustRegister(t, p, HostFunction{Name: "alpha", Handler: noopH})
	mustRegister(t, p, HostFunction{Name: "middle", Handler: noopH})

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
	mustRegister(t, p, HostFunction{Name: "a", Handler: noopH})
	mustRegister(t, p, HostFunction{Name: "b", Handler: noopH})

	if p.Len() != 2 {
		t.Fatalf("expected 2, got %d", p.Len())
	}
}

func TestProviderErrorOnEmptyName(t *testing.T) {
	p := NewHostFunctionProvider()
	err := p.Register(HostFunction{
		Name:    "",
		Handler: func(ctx context.Context, pluginID string, request []byte) ([]byte, error) { return nil, nil },
	})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestProviderErrorOnNilHandler(t *testing.T) {
	p := NewHostFunctionProvider()
	err := p.Register(HostFunction{
		Name:    "test",
		Handler: nil,
	})
	if err == nil {
		t.Fatal("expected error for nil handler")
	}
}

func TestProviderErrorOnDuplicate(t *testing.T) {
	p := NewHostFunctionProvider()
	noopH := func(ctx context.Context, pluginID string, request []byte) ([]byte, error) {
		return nil, nil
	}
	mustRegister(t, p, HostFunction{Name: "dup", Handler: noopH})

	err := p.Register(HostFunction{Name: "dup", Handler: noopH})
	if err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}
