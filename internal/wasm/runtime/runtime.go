package runtime

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// Runtime wraps wazero and provides Wasm module compilation and instantiation.
type Runtime struct {
	engine wazero.Runtime
	cache  wazero.CompilationCache
	config Config
}

// NewRuntime creates a new Wasm runtime with optional AOT compilation cache on disk.
func NewRuntime(ctx context.Context, cfg Config) (*Runtime, error) {
	cfg = cfg.withDefaults()

	rtCfg := wazero.NewRuntimeConfig().
		WithCloseOnContextDone(true).
		WithMemoryLimitPages(cfg.DefaultMemoryLimitPages)

	var cache wazero.CompilationCache
	if cfg.CacheDir != "" {
		var err error
		cache, err = wazero.NewCompilationCacheWithDir(cfg.CacheDir)
		if err != nil {
			return nil, fmt.Errorf("create compilation cache at %q: %w", cfg.CacheDir, err)
		}
		rtCfg = rtCfg.WithCompilationCache(cache)
		slog.Info("wasm: AOT compilation cache enabled", "dir", cfg.CacheDir)
	}

	engine := wazero.NewRuntimeWithConfig(ctx, rtCfg)

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, engine); err != nil {
		return nil, fmt.Errorf("instantiate wasi_snapshot_preview1: %w", err)
	}

	return &Runtime{
		engine: engine,
		cache:  cache,
		config: cfg,
	}, nil
}

// Engine returns the underlying wazero.Runtime for host module registration.
func (r *Runtime) Engine() wazero.Runtime {
	return r.engine
}

// Config returns the runtime configuration.
func (r *Runtime) Config() Config {
	return r.config
}

// Close releases all runtime resources.
func (r *Runtime) Close(ctx context.Context) error {
	var firstErr error
	if err := r.engine.Close(ctx); err != nil {
		firstErr = err
	}
	if r.cache != nil {
		if cErr := r.cache.Close(ctx); cErr != nil && firstErr == nil {
			firstErr = cErr
		}
	}
	return firstErr
}
