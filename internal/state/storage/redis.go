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
	// dialogKeyPrefix is the Redis key prefix for dialog states.
	dialogKeyPrefix = "dialog:state:"

	// defaultTTL is the default time-to-live for dialog state entries.
	// After this duration without updates, the state is automatically removed.
	defaultTTL = 30 * time.Minute
)

// RedisStorage implements DialogStorage using Redis as the backing store.
type RedisStorage struct {
	client *redis.Client
	ttl    time.Duration
}

// RedisOption configures a RedisStorage.
type RedisOption func(*RedisStorage)

// WithTTL sets a custom TTL for dialog state entries.
func WithTTL(ttl time.Duration) RedisOption {
	return func(r *RedisStorage) {
		r.ttl = ttl
	}
}

// NewRedisStorage creates a new RedisStorage with the given Redis client.
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

// Save persists the dialog state as JSON in Redis with a TTL.
func (r *RedisStorage) Save(ctx context.Context, userID model.GlobalUserID, state model.DialogState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshaling dialog state: %w", err)
	}
	return r.client.Set(ctx, r.key(userID), data, r.ttl).Err()
}

// Load retrieves and deserializes the dialog state from Redis.
// Returns (nil, nil) when the key does not exist.
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

// Delete removes the dialog state from Redis.
func (r *RedisStorage) Delete(ctx context.Context, userID model.GlobalUserID) error {
	return r.client.Del(ctx, r.key(userID)).Err()
}

// Ensure RedisStorage implements DialogStorage at compile time.
var _ DialogStorage = (*RedisStorage)(nil)
