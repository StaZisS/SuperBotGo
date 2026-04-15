package trigger

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"SuperBotGo/internal/metrics"
	"SuperBotGo/internal/model"
	wasmrt "SuperBotGo/internal/wasm/runtime"

	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
)

const (
	cronTriggerTimeout  = 30 * time.Second
	cronLockGranularity = 60 // seconds — one lock per minute window
	cronLockTTL         = 2 * time.Minute
)

type cronEntry struct {
	EntryID     cron.EntryID
	TriggerName string
	Schedule    string
}

type CronScheduler struct {
	mu      sync.Mutex
	cron    *cron.Cron
	entries map[string][]cronEntry
	router  *Router
	redis   *redis.Client
	metrics *metrics.Metrics

	running sync.Map
}

func NewCronScheduler(router *Router) *CronScheduler {
	return &CronScheduler{
		cron:    cron.New(),
		entries: make(map[string][]cronEntry),
		router:  router,
	}
}

func (cs *CronScheduler) SetRedis(rc *redis.Client) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.redis = rc
}

func (cs *CronScheduler) SetMetrics(metricSet *metrics.Metrics) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.metrics = metricSet
}

func (cs *CronScheduler) AddSchedule(pluginID, triggerName, schedule string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	entryID, err := cs.cron.AddFunc(schedule, func() {
		cs.fire(pluginID, triggerName)
	})
	if err != nil {
		return err
	}

	cs.entries[pluginID] = append(cs.entries[pluginID], cronEntry{
		EntryID:     entryID,
		TriggerName: triggerName,
		Schedule:    schedule,
	})

	slog.Info("cron: schedule added",
		"plugin", pluginID,
		"trigger", triggerName,
		"schedule", schedule,
	)
	return nil
}

func (cs *CronScheduler) RemoveAll(pluginID string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for _, e := range cs.entries[pluginID] {
		cs.cron.Remove(e.EntryID)
		cs.running.Delete(cronRunKey(pluginID, e.TriggerName))
	}
	delete(cs.entries, pluginID)
}

func cronRunKey(pluginID, triggerName string) string {
	return pluginID + ":" + triggerName
}

func (cs *CronScheduler) Start() {
	cs.cron.Start()
	slog.Info("cron: scheduler started")
}

func (cs *CronScheduler) Stop() {
	ctx := cs.cron.Stop()
	<-ctx.Done()
	slog.Info("cron: scheduler stopped")
}

func (cs *CronScheduler) tryLock(pluginID, triggerName string, fireTime time.Time) bool {
	cs.mu.Lock()
	rc := cs.redis
	cs.mu.Unlock()

	if rc == nil {
		return true
	}

	key := fmt.Sprintf("cron_lock:%s:%s:%d", pluginID, triggerName, fireTime.Unix()/cronLockGranularity)

	ok, err := rc.SetNX(context.Background(), key, 1, cronLockTTL).Result()
	if err != nil {
		slog.Warn("cron: redis lock failed, executing anyway",
			"plugin", pluginID,
			"trigger", triggerName,
			"error", err,
		)
		return true
	}
	return ok
}

func (cs *CronScheduler) fire(pluginID, triggerName string) {
	runKey := cronRunKey(pluginID, triggerName)
	if _, alreadyRunning := cs.running.LoadOrStore(runKey, struct{}{}); alreadyRunning {
		cs.incTrigger(pluginID, triggerName, "skipped_running")
		slog.Warn(fmt.Sprintf("skipping cron trigger %s: previous execution still running", triggerName),
			"plugin", pluginID,
			"trigger", triggerName,
		)
		return
	}
	defer cs.running.Delete(runKey)

	now := time.Now()

	if !cs.tryLock(pluginID, triggerName, now) {
		cs.incTrigger(pluginID, triggerName, "skipped_locked")
		slog.Debug("cron: skipped (another instance holds the lock)",
			"plugin", pluginID,
			"trigger", triggerName,
		)
		return
	}

	data, err := json.Marshal(model.CronTriggerData{
		ScheduleName: triggerName,
		FireTime:     now.UnixMilli(),
	})
	if err != nil {
		cs.incTrigger(pluginID, triggerName, "marshal_error")
		slog.Error("cron: marshal trigger data failed",
			"plugin", pluginID,
			"trigger", triggerName,
			"error", err,
		)
		return
	}

	event := model.Event{
		ID:          generateID(),
		TriggerType: model.TriggerCron,
		TriggerName: triggerName,
		PluginID:    pluginID,
		Timestamp:   now.UnixMilli(),
		Data:        data,
	}

	ctx, cancel := context.WithTimeout(context.Background(), cronTriggerTimeout)
	defer cancel()
	ctx = context.WithValue(ctx, wasmrt.PluginTimeoutOverrideKey{}, int(cronTriggerTimeout.Seconds()))

	resp, err := cs.router.RouteEvent(ctx, event)
	if err != nil {
		cs.incTrigger(pluginID, triggerName, "dispatch_error")
		slog.Error("cron: dispatch failed",
			"plugin", pluginID,
			"trigger", triggerName,
			"error", err,
		)
		return
	}
	if resp != nil && resp.Error != "" {
		cs.incTrigger(pluginID, triggerName, "plugin_error")
		slog.Error("cron: plugin returned error",
			"plugin", pluginID,
			"trigger", triggerName,
			"error", resp.Error,
		)
		return
	}
	cs.incTrigger(pluginID, triggerName, "ok")
}

func (cs *CronScheduler) incTrigger(pluginID, triggerName, result string) {
	cs.mu.Lock()
	metricSet := cs.metrics
	cs.mu.Unlock()
	if metricSet == nil {
		return
	}
	metricSet.CronTriggerTotal.WithLabelValues(pluginID, triggerName, result).Inc()
}
