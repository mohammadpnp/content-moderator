package redis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/outbound"
	"github.com/redis/go-redis/v9"
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

type RedisCacheStore struct {
	client *redis.Client
}

func NewCacheStore(client *redis.Client) *RedisCacheStore {
	return &RedisCacheStore{client: client}
}

var _ outbound.CacheStore = (*RedisCacheStore)(nil)

func (r *RedisCacheStore) GetModerationResult(ctx context.Context, contentID string) (*entity.ModerationResult, error) {
	data, err := r.client.Get(ctx, contentID).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("cache miss for content: %s", contentID)
		}
		return nil, fmt.Errorf("redis get error: %w", err)
	}

	var result entity.ModerationResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("cache unmarshal error: %w", err)
	}
	return &result, nil
}

func (r *RedisCacheStore) SetModerationResult(ctx context.Context, contentID string, result *entity.ModerationResult, ttl time.Duration) error {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	if err := json.NewEncoder(buf).Encode(result); err != nil {
		return fmt.Errorf("cache marshal error: %w", err)
	}

	if err := r.client.Set(ctx, contentID, buf.Bytes(), ttl).Err(); err != nil {
		return fmt.Errorf("redis set error: %w", err)
	}
	return nil
}

func (r *RedisCacheStore) Invalidate(ctx context.Context, key string) error {
	if err := r.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("redis del error: %w", err)
	}
	return nil
}

func (r *RedisCacheStore) SetIfNotExists(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	if err := json.NewEncoder(buf).Encode(value); err != nil {
		return false, fmt.Errorf("marshal error: %w", err)
	}

	ok, err := r.client.SetNX(ctx, key, buf.Bytes(), ttl).Result()
	if err != nil {
		return false, fmt.Errorf("redis setnx error: %w", err)
	}
	return ok, nil
}
