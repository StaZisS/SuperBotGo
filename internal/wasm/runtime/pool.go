package runtime

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/sys"
)

type ModulePool struct {
	compiled *CompiledModule
	sem      chan struct{}
	poolSize int

	totalExecutions   atomic.Int64
	activeExecutions  atomic.Int64
	totalWaitTime     atomic.Int64
	totalInstantiateT atomic.Int64

	closed atomic.Bool
	mu     sync.Mutex
}

type PoolConfig struct {
	MaxConcurrency int
}

const DefaultPoolConcurrency = 8

func NewModulePool(cm *CompiledModule, cfg *PoolConfig) *ModulePool {
	size := DefaultPoolConcurrency
	if cfg != nil && cfg.MaxConcurrency > 0 {
		size = cfg.MaxConcurrency
	}

	var sem chan struct{}
	if cfg == nil || cfg.MaxConcurrency >= 0 {
		sem = make(chan struct{}, size)
		for i := 0; i < size; i++ {
			sem <- struct{}{}
		}
	}

	return &ModulePool{
		compiled: cm,
		sem:      sem,
		poolSize: size,
	}
}

type PoolStats struct {
	PoolSize         int
	ActiveExecutions int64
	TotalExecutions  int64
	AvgWaitTimeMs    float64
	AvgInstantiateMs float64
}

func (p *ModulePool) Stats() PoolStats {
	total := p.totalExecutions.Load()
	var avgWait, avgInst float64
	if total > 0 {
		avgWait = float64(p.totalWaitTime.Load()) / float64(total) / 1e6
		avgInst = float64(p.totalInstantiateT.Load()) / float64(total) / 1e6
	}
	return PoolStats{
		PoolSize:         p.poolSize,
		ActiveExecutions: p.activeExecutions.Load(),
		TotalExecutions:  total,
		AvgWaitTimeMs:    avgWait,
		AvgInstantiateMs: avgInst,
	}
}

func (p *ModulePool) Execute(ctx context.Context, action string, input []byte, configJSON []byte) ([]byte, error) {
	if p.closed.Load() {
		return nil, fmt.Errorf("module pool is closed")
	}

	cm := p.compiled

	timeoutSec := pluginTimeoutFromContext(ctx, cm.rt.config.DefaultTimeoutSeconds)
	timeout := time.Duration(timeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ctx = context.WithValue(ctx, PluginIDKey{}, cm.ID)

	for _, hook := range cm.rt.contextHooks {
		ctx = hook(ctx, cm.ID)
	}

	start := time.Now()
	status := "ok"
	defer func() {
		duration := time.Since(start)
		if cm.rt.metrics != nil {
			cm.rt.metrics.PluginActionDuration.WithLabelValues(cm.ID, action).Observe(duration.Seconds())
			cm.rt.metrics.PluginActionTotal.WithLabelValues(cm.ID, action, status).Inc()
		}
		slog.Info("plugin action (pooled)",
			"plugin_id", cm.ID,
			"action", action,
			"duration_ms", duration.Milliseconds(),
			"status", status,
			"pool_active", p.activeExecutions.Load(),
		)
	}()

	waitStart := time.Now()
	if p.sem != nil {
		select {
		case <-p.sem:
		case <-ctx.Done():
			status = "timeout_waiting"
			return nil, fmt.Errorf("context cancelled waiting for pool slot: %w", ctx.Err())
		}
		defer func() { p.sem <- struct{}{} }()
	}
	waitDuration := time.Since(waitStart)
	p.totalWaitTime.Add(waitDuration.Nanoseconds())

	p.activeExecutions.Add(1)
	defer p.activeExecutions.Add(-1)
	p.totalExecutions.Add(1)

	var stdout bytes.Buffer
	var stdin *bytes.Reader
	if len(input) > 0 {
		stdin = bytes.NewReader(input)
	} else {
		stdin = bytes.NewReader(nil)
	}

	var stderr io.Writer = io.Discard
	if cm.ID != "" {
		stderr = &slogWriter{pluginID: cm.ID}
	}

	modCfg := wazero.NewModuleConfig().
		WithEnv("PLUGIN_ACTION", action).
		WithStdin(stdin).
		WithStdout(&stdout).
		WithStderr(stderr).
		WithFSConfig(wazero.NewFSConfig()).
		WithName("")

	if len(configJSON) > 0 {
		modCfg = modCfg.WithEnv("PLUGIN_CONFIG", string(configJSON))
	}

	instStart := time.Now()
	_, err := cm.rt.engine.InstantiateModule(ctx, cm.compiled, modCfg)
	instDuration := time.Since(instStart)
	p.totalInstantiateT.Add(instDuration.Nanoseconds())

	if err != nil {
		if exitErr, ok := err.(*sys.ExitError); ok {
			if exitErr.ExitCode() == 0 {
				return stdout.Bytes(), nil
			}
			status = "error"
			return nil, fmt.Errorf("wasm module exited with code %d", exitErr.ExitCode())
		}
		status = "error"
		return nil, fmt.Errorf("instantiate wasm module: %w", err)
	}

	return stdout.Bytes(), nil
}

func (p *ModulePool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed.Load() {
		return
	}
	p.closed.Store(true)

	if p.sem != nil {
		for i := 0; i < p.poolSize; i++ {
			<-p.sem
		}
	}
}
