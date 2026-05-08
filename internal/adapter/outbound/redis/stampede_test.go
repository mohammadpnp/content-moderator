package redis_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	redisadapter "github.com/mohammadpnp/content-moderator/internal/adapter/outbound/redis"
	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupStampede یک RedisCacheStore با miniredis می‌سازه و تابع cleanup برمی‌گردونه.
func setupStampede(t *testing.T) (*redisadapter.RedisCacheStore, func()) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := redisadapter.NewCacheStore(client)
	return store, func() { mr.Close(); client.Close() }
}

// TestWithCacheLock_StampedeProtection تست می‌کنه که با ۵۰ درخواست همزمان
// فقط یک بار تابع fetchFn اجرا بشه (Cache Stampede Protection).
func TestWithCacheLock_StampedeProtection(t *testing.T) {
	store, cleanup := setupStampede(t)
	defer cleanup()

	var mu sync.Mutex
	callCount := 0

	// تابعی که کار سنگین (مثلاً خوندن از دیتابیس) رو شبیه‌سازی می‌کنه.
	fetchFn := func() (interface{}, error) {
		mu.Lock()
		callCount++
		mu.Unlock()
		time.Sleep(10 * time.Millisecond) // simulate work
		return &entity.ModerationResult{
			ID:         "res-1",
			ContentID:  "c1",
			IsApproved: true,
			Score:      0.9,
		}, nil
	}

	key := "stampede-key"
	ttl := 10 * time.Second
	lockTTL := 5 * time.Second
	maxRetries := 5
	retryDelay := 20 * time.Millisecond

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			_, err := redisadapter.WithCacheLock(
				context.Background(),
				store,
				key,
				ttl,
				lockTTL,
				maxRetries,
				retryDelay,
				fetchFn,
			)
			assert.NoError(t, err)
		}()
	}

	wg.Wait()

	// اعتبارسنجی: fetchFn باید دقیقاً یک بار فراخوانی شده باشه.
	mu.Lock()
	assert.Equal(t, 1, callCount, "fetchFn must be called exactly once (stampede protection)")
	mu.Unlock()

	// بررسی اینکه داده در کش ذخیره شده.
	cached, err := store.GetModerationResult(context.Background(), key)
	require.NoError(t, err)
	assert.Equal(t, "res-1", cached.ID)
}

// TestWithCacheLock_CacheHit تست می‌کنه که اگر داده از قبل در کش باشه،
// fetchFn اصلاً فراخوانی نشه.
func TestWithCacheLock_CacheHit(t *testing.T) {
	store, cleanup := setupStampede(t)
	defer cleanup()

	key := "hit-key"
	ttl := 10 * time.Second

	// یک داده از قبل توی کش می‌ذاریم
	preloaded := &entity.ModerationResult{
		ID:         "existing",
		ContentID:  "c1",
		IsApproved: true,
		Score:      1.0,
	}
	err := store.SetModerationResult(context.Background(), key, preloaded, ttl)
	require.NoError(t, err)

	fetchFn := func() (interface{}, error) {
		t.Error("fetchFn should not be called on cache hit")
		return nil, nil
	}

	res, err := redisadapter.WithCacheLock(
		context.Background(),
		store,
		key,
		ttl,
		5*time.Second,
		5,
		20*time.Millisecond,
		fetchFn,
	)
	assert.NoError(t, err)
	modRes, ok := res.(*entity.ModerationResult)
	require.True(t, ok)
	assert.Equal(t, "existing", modRes.ID)
}
