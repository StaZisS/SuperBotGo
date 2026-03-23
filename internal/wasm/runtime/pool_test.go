package runtime

import (
	"sync"
	"testing"
)

func TestNewModulePool_DefaultConcurrency(t *testing.T) {
	// nil config should use DefaultPoolConcurrency.
	p := NewModulePool(&CompiledModule{rt: &Runtime{config: Config{}.withDefaults()}}, nil)
	stats := p.Stats()
	if stats.PoolSize != DefaultPoolConcurrency {
		t.Errorf("expected pool size %d, got %d", DefaultPoolConcurrency, stats.PoolSize)
	}
	if stats.TotalExecutions != 0 {
		t.Errorf("expected 0 total executions, got %d", stats.TotalExecutions)
	}
	if stats.ActiveExecutions != 0 {
		t.Errorf("expected 0 active executions, got %d", stats.ActiveExecutions)
	}
	p.Close()
}

func TestNewModulePool_CustomConcurrency(t *testing.T) {
	p := NewModulePool(
		&CompiledModule{rt: &Runtime{config: Config{}.withDefaults()}},
		&PoolConfig{MaxConcurrency: 16},
	)
	stats := p.Stats()
	if stats.PoolSize != 16 {
		t.Errorf("expected pool size 16, got %d", stats.PoolSize)
	}
	p.Close()
}

func TestNewModulePool_UnlimitedConcurrency(t *testing.T) {
	// Negative MaxConcurrency means unlimited (no semaphore).
	p := NewModulePool(
		&CompiledModule{rt: &Runtime{config: Config{}.withDefaults()}},
		&PoolConfig{MaxConcurrency: -1},
	)
	if p.sem != nil {
		t.Error("expected nil semaphore for unlimited concurrency")
	}
	p.Close()
}

func TestModulePool_CloseIdempotent(t *testing.T) {
	p := NewModulePool(&CompiledModule{rt: &Runtime{config: Config{}.withDefaults()}}, nil)
	p.Close()
	p.Close() // should not panic or deadlock
	if !p.closed.Load() {
		t.Error("expected pool to be closed")
	}
}

func TestModulePool_SemaphoreLimit(t *testing.T) {
	// Verify that the semaphore allows exactly MaxConcurrency goroutines.
	const maxConc = 3
	p := NewModulePool(
		&CompiledModule{rt: &Runtime{config: Config{}.withDefaults()}},
		&PoolConfig{MaxConcurrency: maxConc},
	)
	defer p.Close()

	// Acquire all slots manually.
	for i := 0; i < maxConc; i++ {
		select {
		case <-p.sem:
			// ok
		default:
			t.Fatalf("should be able to acquire slot %d", i)
		}
	}

	// The next acquire should block (channel empty).
	select {
	case <-p.sem:
		t.Fatal("should not be able to acquire beyond max concurrency")
	default:
		// expected
	}

	// Return all slots.
	for i := 0; i < maxConc; i++ {
		p.sem <- struct{}{}
	}
}

func TestModulePool_StatsAfterClose(t *testing.T) {
	p := NewModulePool(&CompiledModule{rt: &Runtime{config: Config{}.withDefaults()}}, nil)
	// Simulate some stats.
	p.totalExecutions.Store(42)
	p.totalWaitTime.Store(1_000_000)     // 1ms total
	p.totalInstantiateT.Store(5_000_000) // 5ms total
	p.Close()

	stats := p.Stats()
	if stats.TotalExecutions != 42 {
		t.Errorf("expected 42 total executions, got %d", stats.TotalExecutions)
	}
	// avgWait = 1_000_000ns / 42 / 1e6 ≈ 0.0238 ms
	if stats.AvgWaitTimeMs < 0.02 || stats.AvgWaitTimeMs > 0.03 {
		t.Errorf("unexpected avg wait time: %f ms", stats.AvgWaitTimeMs)
	}
	// avgInst = 5_000_000ns / 42 / 1e6 ≈ 0.119 ms
	if stats.AvgInstantiateMs < 0.1 || stats.AvgInstantiateMs > 0.13 {
		t.Errorf("unexpected avg instantiate time: %f ms", stats.AvgInstantiateMs)
	}
}

func TestModulePool_ConcurrentClose(t *testing.T) {
	// Calling Close from multiple goroutines should not panic.
	p := NewModulePool(
		&CompiledModule{rt: &Runtime{config: Config{}.withDefaults()}},
		&PoolConfig{MaxConcurrency: 4},
	)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.Close()
		}()
	}
	wg.Wait()

	if !p.closed.Load() {
		t.Error("expected pool to be closed after concurrent Close calls")
	}
}

func TestConfigPoolConfig(t *testing.T) {
	// Zero PoolMaxConcurrency returns nil (use defaults).
	cfg := Config{}
	if cfg.PoolConfig() != nil {
		t.Error("expected nil PoolConfig for zero PoolMaxConcurrency")
	}

	// Positive value returns a PoolConfig.
	cfg = Config{PoolMaxConcurrency: 12}
	pc := cfg.PoolConfig()
	if pc == nil {
		t.Fatal("expected non-nil PoolConfig")
	}
	if pc.MaxConcurrency != 12 {
		t.Errorf("expected MaxConcurrency 12, got %d", pc.MaxConcurrency)
	}

	// Negative value returns a PoolConfig (unlimited).
	cfg = Config{PoolMaxConcurrency: -1}
	pc = cfg.PoolConfig()
	if pc == nil {
		t.Fatal("expected non-nil PoolConfig for negative PoolMaxConcurrency")
	}
	if pc.MaxConcurrency != -1 {
		t.Errorf("expected MaxConcurrency -1, got %d", pc.MaxConcurrency)
	}
}

func TestCompiledModule_EnablePool(t *testing.T) {
	cm := &CompiledModule{rt: &Runtime{config: Config{}.withDefaults()}}

	if cm.Pool() != nil {
		t.Error("pool should be nil before EnablePool")
	}

	cm.EnablePool(nil)
	if cm.Pool() == nil {
		t.Fatal("pool should not be nil after EnablePool")
	}

	stats := cm.Pool().Stats()
	if stats.PoolSize != DefaultPoolConcurrency {
		t.Errorf("expected default pool size %d, got %d", DefaultPoolConcurrency, stats.PoolSize)
	}

	// EnablePool again should close the old pool and create a new one.
	oldPool := cm.Pool()
	cm.EnablePool(&PoolConfig{MaxConcurrency: 2})
	if oldPool == cm.Pool() {
		t.Error("expected new pool after re-enabling")
	}
	if cm.Pool().Stats().PoolSize != 2 {
		t.Errorf("expected pool size 2, got %d", cm.Pool().Stats().PoolSize)
	}
}
