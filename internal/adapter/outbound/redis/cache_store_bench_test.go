package redis_test

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	redisadapter "github.com/mohammadpnp/content-moderator/internal/adapter/outbound/redis"
	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/redis/go-redis/v9"
)

// ============================================================
// ۱. JSON Marshal – Pool vs No Pool
// ============================================================
func BenchmarkJSONMarshal_NoPool(b *testing.B) {
	result := sampleModerationResult()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(result)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONMarshal_WithPool(b *testing.B) {
	result := sampleModerationResult()
	bufPool := &sync.Pool{New: func() any { return new(bytes.Buffer) }}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := bufPool.Get().(*bytes.Buffer)
		buf.Reset()
		if err := json.NewEncoder(buf).Encode(result); err != nil {
			b.Fatal(err)
		}
		_ = buf.Bytes()
		bufPool.Put(buf)
	}
}

// ============================================================
// ۲. RedisCacheStore Set + Get (with Pool) – Redis واقعی یا Miniredis
// ============================================================
func BenchmarkCacheSet(b *testing.B) {
	cache, cleanup := setupBenchCache(b)
	defer cleanup()

	ctx := context.Background()
	result := sampleModerationResult()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := cache.SetModerationResult(ctx, "bench-key", result, 10*time.Minute); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCacheGet(b *testing.B) {
	cache, cleanup := setupBenchCache(b)
	defer cleanup()

	ctx := context.Background()
	result := sampleModerationResult()
	// Pre-populate cache
	if err := cache.SetModerationResult(ctx, "bench-key", result, 10*time.Minute); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := cache.GetModerationResult(ctx, "bench-key"); err != nil {
			b.Fatal(err)
		}
	}
}

// ============================================================
// ۳. Cache-Aside کامل: GetModerationResult با منطق fallback (در service)
//
//	شبیه‌سازی‌شده: اگر کش خالی بود، از "دیتابیس" mock بخونه
//	ولی اینجا فقط خود GetModerationResult (کش) رو تست می‌کنیم.
//
// ============================================================
func BenchmarkCacheAside_Hit(b *testing.B) {
	cache, cleanup := setupBenchCache(b)
	defer cleanup()

	ctx := context.Background()
	result := sampleModerationResult()
	cache.SetModerationResult(ctx, "hit", result, 10*time.Minute)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := cache.GetModerationResult(ctx, "hit"); err != nil {
			b.Fatal(err)
		}
	}
}

// ============================================================
// ۴. Distributed Lock (WITHOUT Redis – فقط منطق قفل با mock)
// ============================================================
// برای تست قفل می‌تونیم یک تست مجزا بنویسم، نه لزوماً benchmark
// ============================================================

// -------------------- helpers --------------------
func sampleModerationResult() *entity.ModerationResult {
	return &entity.ModerationResult{
		ID:         "result-1",
		ContentID:  "content-1",
		IsApproved: true,
		Score:      0.95,
		Categories: []entity.ModerationCategory{"spam", "hate"},
		ModelName:  "mock-bert",
		DurationMs: 150,
	}
}

func setupBenchCache(b *testing.B) (*redisadapter.RedisCacheStore, func()) {
	b.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		b.Fatal(err)
	}
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cache := redisadapter.NewCacheStore(client)
	return cache, func() { mr.Close(); client.Close() }
}
