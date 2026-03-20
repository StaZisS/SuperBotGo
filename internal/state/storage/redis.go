package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"SuperBotGo/internal/model"

	"github.com/redis/go-redis/v9"
)

const (
	dialogKeyPrefix = "dialog:state:"

	defaultTTL = 30 * time.Minute
)

type RedisStorage struct {
	client *redis.Client
	ttl    time.Duration
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

func (r *RedisStorage) key(userID model.GlobalUserID) string {
	return fmt.Sprintf("%s%d", dialogKeyPrefix, userID)
}

func (r *RedisStorage) Save(ctx context.Context, userID model.GlobalUserID, state model.DialogState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshaling dialog state: %w", err)
	}
	return r.client.Set(ctx, r.key(userID), data, r.ttl).Err()
}

func (r *RedisStorage) Load(ctx context.Context, userID model.GlobalUserID) (*model.DialogState, error) {
	data, err := r.client.Get(ctx, r.key(userID)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("loading dialog state: %w", err)
	}

	var state model.DialogState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshaling dialog state: %w", err)
	}
	return &state, nil
}

func (r *RedisStorage) Delete(ctx context.Context, userID model.GlobalUserID) error {
	return r.client.Del(ctx, r.key(userID)).Err()
}

var _ DialogStorage = (*RedisStorage)(nil)
