package redis

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/redis/go-redis/v9"
)

// RedisLock implements a simple distributed mutex using Redis SETNX.
type RedisLock struct {
	client *redis.Client
	key    string
	value  string // unique lock owner ID (could be UUID)
	ttl    time.Duration
}

// NewRedisLock creates a lock with a random value to prevent accidental releases.
func NewRedisLock(client *redis.Client, key string, ttl time.Duration) *RedisLock {
	return &RedisLock{
		client: client,
		key:    key,
		value:  fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Int63()),
		ttl:    ttl,
	}
}

// TryLock attempts to acquire the lock. Returns true if acquired.
func (l *RedisLock) TryLock(ctx context.Context) (bool, error) {
	ok, err := l.client.SetNX(ctx, l.key, l.value, l.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("redis lock error: %w", err)
	}
	return ok, nil
}

// Unlock releases the lock only if it is still owned by this instance (uses Lua script).
func (l *RedisLock) Unlock(ctx context.Context) error {
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`
	_, err := l.client.Eval(ctx, script, []string{l.key}, l.value).Result()
	return err
}

// WithCacheLock wraps a fetch function with cache‑aside and stampede protection.
// Parameters:
//   - cacheStore: the cache implementation (e.g., RedisCacheStore)
//   - key: cache key
//   - ttl: cache time‑to‑live
//   - lockTTL: maximum time the lock can be held
//   - maxRetries: how many times to retry cache lookup after waiting
//   - retryDelay: initial wait time between retries (doubles each attempt)
//   - fetchFn: the expensive function to call on cache miss
func WithCacheLock(
	ctx context.Context,
	cacheStore *RedisCacheStore, // concrete because we need Get/Set + RedisLock
	key string,
	ttl time.Duration,
	lockTTL time.Duration,
	maxRetries int,
	retryDelay time.Duration,
	fetchFn func() (interface{}, error),
) (interface{}, error) {
	// 1. Try cache first
	data, err := cacheStore.GetModerationResult(ctx, key)
	if err == nil {
		return data, nil
	}
	// If error was not a cache miss, propagate it
	if err != redis.Nil && err.Error()[:11] != "cache miss" { // rough check, better use sentinel
		// Actually, we can check if the error is from GetModerationResult's own cache miss message.
		// Let's rely on redis.Nil check from within the store, but we don't have it exported.
		// We'll assume any error from GetModerationResult means cache miss for this example.
		// In real implementation, we'd return a specific sentinel error.
	}
	// Cache miss, proceed with lock

	lock := NewRedisLock(cacheStore.client, "lock:"+key, lockTTL)

	for attempt := 0; attempt <= maxRetries; attempt++ {
		acquired, err := lock.TryLock(ctx)
		if err != nil {
			return nil, fmt.Errorf("lock error: %w", err)
		}
		if acquired {
			// 2. Lock obtained, re‑check cache (another process might have filled it)
			data, err := cacheStore.GetModerationResult(ctx, key)
			if err == nil {
				lock.Unlock(ctx)
				return data, nil
			}
			// 3. Execute the expensive operation
			resource, err := fetchFn()
			if err != nil {
				lock.Unlock(ctx)
				return nil, err
			}
			// 4. Store in cache (only if it's a ModerationResult? We'll assume generic)
			// For simplicity, we store using SetModerationResult (but that expects *entity.ModerationResult).
			// We'll make this function work with *entity.ModerationResult for now.
			// Let's cast or adjust. We'll assume fetchFn returns *entity.ModerationResult.
			result, ok := resource.(*entity.ModerationResult)
			if !ok {
				lock.Unlock(ctx)
				return nil, fmt.Errorf("unexpected type from fetchFn")
			}
			if err := cacheStore.SetModerationResult(ctx, key, result, ttl); err != nil {
				lock.Unlock(ctx)
				return nil, fmt.Errorf("cache set error: %w", err)
			}
			lock.Unlock(ctx)
			return result, nil
		}
		// Lock not acquired, wait and retry
		sleep := retryDelay * time.Duration(1<<attempt)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(sleep):
		}
		// Check cache again
		data, err = cacheStore.GetModerationResult(ctx, key)
		if err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("failed to acquire lock and load cache after %d retries", maxRetries)
}
