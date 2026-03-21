package trigger

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"SuperBotGo/internal/model"

	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
)

// cronEntry tracks a single cron job so it can be removed later.
type cronEntry struct {
	EntryID     cron.EntryID
	TriggerName string
	Schedule    string
}

// CronScheduler manages cron jobs for plugins and dispatches events
// through the Router when they fire. When a Redis client is set,
// it uses distributed locking (SET NX) so that only one instance
// executes a given cron tick across a multi-instance deployment.
type CronScheduler struct {
	mu      sync.Mutex
	cron    *cron.Cron
	entries map[string][]cronEntry // pluginID -> entries
	router  *Router
	redis   *redis.Client
}

func NewCronScheduler(router *Router) *CronScheduler {
	return &CronScheduler{
		cron:    cron.New(),
		entries: make(map[string][]cronEntry),
		router:  router,
	}
}

// SetRedis sets the Redis client used for distributed cron locking.
// Without Redis every instance fires independently.
func (cs *CronScheduler) SetRedis(rc *redis.Client) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.redis = rc
}

// AddSchedule registers a cron trigger for a plugin.
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

// RemoveAll removes all cron entries for a plugin.
func (cs *CronScheduler) RemoveAll(pluginID string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for _, e := range cs.entries[pluginID] {
		cs.cron.Remove(e.EntryID)
	}
	delete(cs.entries, pluginID)
}

// Start starts the cron scheduler.
func (cs *CronScheduler) Start() {
	cs.cron.Start()
	slog.Info("cron: scheduler started")
}

// Stop stops the cron scheduler and waits for running jobs to finish.
func (cs *CronScheduler) Stop() {
	ctx := cs.cron.Stop()
	<-ctx.Done()
	slog.Info("cron: scheduler stopped")
}

// tryLock attempts to acquire a distributed lock for this cron tick.
// The key encodes the plugin, trigger, and the minute the job fired
// so that the same tick is only executed once across all instances.
// Returns true if this instance won the lock (or Redis is unavailable).
func (cs *CronScheduler) tryLock(pluginID, triggerName string, fireTime time.Time) bool {
	cs.mu.Lock()
	rc := cs.redis
	cs.mu.Unlock()

	if rc == nil {
		return true // no Redis — single-instance mode
	}

	// Key is unique per trigger per minute (cron granularity is 1 min).
	key := fmt.Sprintf("cron_lock:%s:%s:%d", pluginID, triggerName, fireTime.Unix()/60)
	ttl := 2 * time.Minute // hold slightly longer than the cron period

	ok, err := rc.SetNX(context.Background(), key, 1, ttl).Result()
	if err != nil {
		// Redis failure — let this instance execute to avoid silent skips.
		slog.Warn("cron: redis lock failed, executing anyway",
			"plugin", pluginID,
			"trigger", triggerName,
			"error", err,
		)
		return true
	}
	return ok
}

// fire builds a model.Event and dispatches it through the router.
func (cs *CronScheduler) fire(pluginID, triggerName string) {
	now := time.Now()

	if !cs.tryLock(pluginID, triggerName, now) {
		slog.Debug("cron: skipped (another instance holds the lock)",
			"plugin", pluginID,
			"trigger", triggerName,
		)
		return
	}

	data, _ := json.Marshal(model.CronTriggerData{
		ScheduleName: triggerName,
		FireTime:     now.UnixMilli(),
	})

	event := model.Event{
		ID:          generateID(),
		TriggerType: model.TriggerCron,
		TriggerName: triggerName,
		PluginID:    pluginID,
		Timestamp:   now.UnixMilli(),
		Data:        data,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := cs.router.RouteEvent(ctx, event)
	if err != nil {
		slog.Error("cron: dispatch failed",
			"plugin", pluginID,
			"trigger", triggerName,
			"error", err,
		)
		return
	}
	if resp != nil && resp.Error != "" {
		slog.Error("cron: plugin returned error",
			"plugin", pluginID,
			"trigger", triggerName,
			"error", resp.Error,
		)
	}
}
