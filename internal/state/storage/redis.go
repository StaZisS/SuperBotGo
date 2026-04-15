package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"SuperBotGo/internal/metrics"
	"SuperBotGo/internal/model"

	"github.com/redis/go-redis/v9"
)

const (
	dialogKeyPrefix = "dialog:state:"

	defaultTTL = 30 * time.Minute
)

type RedisStorage struct {
	client  *redis.Client
	ttl     time.Duration
	metrics *metrics.Metrics
}

type RedisOption func(*RedisStorage)

func WithTTL(ttl time.Duration) RedisOption {
	return func(r *RedisStorage) {
		r.ttl = ttl
	}
}

func NewRedisStorage(client *redis.Client, opts ...RedisOption) *RedisStorage {
	rs := &RedisStorage{
		client: client,
		ttl:    defaultTTL,
	}
	for _, opt := range opts {
		opt(rs)
	}
	return rs
}

func (r *RedisStorage) SetMetrics(metricSet *metrics.Metrics) {
	r.metrics = metricSet
}

func (r *RedisStorage) key(userID model.GlobalUserID) string {
	return fmt.Sprintf("%s%d", dialogKeyPrefix, userID)
}

func (r *RedisStorage) Save(ctx context.Context, userID model.GlobalUserID, state model.DialogState) error {
	start := time.Now()
	result := "ok"
	defer func() {
		r.observe("save", result, time.Since(start))
	}()

	data, err := json.Marshal(state)
	if err != nil {
		result = "marshal_error"
		return fmt.Errorf("marshaling dialog state: %w", err)
	}
	if err := r.client.Set(ctx, r.key(userID), data, r.ttl).Err(); err != nil {
		result = "redis_error"
		return err
	}
	return nil
}

func (r *RedisStorage) Load(ctx context.Context, userID model.GlobalUserID) (*model.DialogState, error) {
	start := time.Now()
	result := "ok"
	defer func() {
		r.observe("load", result, time.Since(start))
	}()

	data, err := r.client.Get(ctx, r.key(userID)).Bytes()
	if err != nil {
		if err == redis.Nil {
			result = "miss"
			return nil, nil
		}
		result = "redis_error"
		return nil, fmt.Errorf("loading dialog state: %w", err)
	}

	var state model.DialogState
	if err := json.Unmarshal(data, &state); err != nil {
		result = "unmarshal_error"
		return nil, fmt.Errorf("unmarshaling dialog state: %w", err)
	}
	return &state, nil
}

func (r *RedisStorage) Delete(ctx context.Context, userID model.GlobalUserID) error {
	start := time.Now()
	result := "ok"
	defer func() {
		r.observe("delete", result, time.Since(start))
	}()

	if err := r.client.Del(ctx, r.key(userID)).Err(); err != nil {
		result = "redis_error"
		return err
	}
	return nil
}

var _ DialogStorage = (*RedisStorage)(nil)

func (r *RedisStorage) observe(op, result string, dur time.Duration) {
	if r.metrics == nil {
		return
	}
	r.metrics.DialogStorageDuration.WithLabelValues(op, result).Observe(dur.Seconds())
}
