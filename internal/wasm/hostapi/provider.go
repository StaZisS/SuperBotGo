package hostapi

import (
	"fmt"
	"sort"
	"sync"
)

type HostFunctionProvider struct {
	mu        sync.RWMutex
	functions map[string]HostFunction
}

func NewHostFunctionProvider() *HostFunctionProvider {
	return &HostFunctionProvider{
		functions: make(map[string]HostFunction),
	}
}

func (p *HostFunctionProvider) Register(fn HostFunction) {
	if fn.Name == "" {
		panic("hostapi: HostFunction.Name must not be empty")
	}
	if fn.Handler == nil {
		panic(fmt.Sprintf("hostapi: HostFunction %q has a nil Handler", fn.Name))
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.functions[fn.Name]; exists {
		panic(fmt.Sprintf("hostapi: duplicate HostFunction registration: %q", fn.Name))
	}
	p.functions[fn.Name] = fn
}

func (p *HostFunctionProvider) Get(name string) (HostFunction, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	fn, ok := p.functions[name]
	return fn, ok
}

func (p *HostFunctionProvider) Functions() []HostFunction {
	p.mu.RLock()
	defer p.mu.RUnlock()

	fns := make([]HostFunction, 0, len(p.functions))
	for _, fn := range p.functions {
		fns = append(fns, fn)
	}
	sort.Slice(fns, func(i, j int) bool {
		return fns[i].Name < fns[j].Name
	})
	return fns
}

func (p *HostFunctionProvider) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.functions)
}
