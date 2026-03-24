package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
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
	p.totalWaitTime.Add(time.Since(waitStart).Nanoseconds())

	p.activeExecutions.Add(1)
	defer p.activeExecutions.Add(-1)
	p.totalExecutions.Add(1)

	instStart := time.Now()
	result, err := cm.instantiate(ctx, action, input, configJSON)
	p.totalInstantiateT.Add(time.Since(instStart).Nanoseconds())

	if err != nil {
		status = "error"
		return nil, err
	}
	return result, nil
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
