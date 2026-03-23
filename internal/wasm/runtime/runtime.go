package runtime

import (
	"context"
	"fmt"
	"log/slog"

	"SuperBotGo/internal/metrics"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type ContextHook func(ctx context.Context, pluginID string) context.Context

type Runtime struct {
	engine       wazero.Runtime
	cache        wazero.CompilationCache
	config       Config
	metrics      *metrics.Metrics
	contextHooks []ContextHook
}

func (r *Runtime) SetMetrics(m *metrics.Metrics) {
	r.metrics = m
}

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

func (r *Runtime) AddContextHook(hook ContextHook) {
	r.contextHooks = append(r.contextHooks, hook)
}

func (r *Runtime) Engine() wazero.Runtime {
	return r.engine
}

func (r *Runtime) Config() Config {
	return r.config
}

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
