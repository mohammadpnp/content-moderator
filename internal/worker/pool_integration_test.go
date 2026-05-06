package worker_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/internal/service"
	"github.com/mohammadpnp/content-moderator/internal/worker"
	"github.com/mohammadpnp/content-moderator/test/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerPool_ProcessesMultipleJobsAndIdempotency(t *testing.T) {
	// Setup mock dependencies with approving AI and fast processing
	repo := mock.NewMockContentRepository()
	aiClient := mock.NewMockAIClient()
	aiClient.SetAsApproving()
	aiClient.SetDelay(0) // no artificial delay for speed
	cache := mock.NewMockCacheStore()
	broker := mock.NewMockMessageBroker()

	modSvc := service.NewModerationService(repo, aiClient, cache, broker)

	// Prepare 10 content items, all pending
	const numJobs = 10
	contents := make([]*entity.Content, numJobs)
	for i := 0; i < numJobs; i++ {
		c, err := entity.NewContent("user1", entity.ContentTypeText, "test message")
		require.NoError(t, err)
		c.ID = "content-" + string(rune('a'+i))
		err = repo.Save(context.Background(), c)
		require.NoError(t, err)
		contents[i] = c
		// Publish job to broker (simulating ContentService)
		err = broker.PublishModerationJob(context.Background(), c)
		require.NoError(t, err)
	}

	// Create worker pool with a single worker for deterministic rate limit testing
	cfg := worker.Config{
		WorkerCount: 1,
		RateLimit:   5, // 5 jobs per second
		RateBurst:   2,
		JobTimeout:  5 * time.Second,
	}
	pool := worker.NewPool(cfg, modSvc, broker)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := pool.Start(ctx)
		// Start returns when context is done; ignore error for test
		_ = err
	}()

	// Wait until all contents are approved
	deadline := time.Now().Add(5 * time.Second)
	for {
		approvedCount := 0
		for _, c := range contents {
			fetched, err := repo.FindByID(context.Background(), c.ID)
			if err == nil && fetched.Status == entity.ContentStatusApproved {
				approvedCount++
			}
		}
		if approvedCount == numJobs {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for all jobs to be processed")
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Verify all contents are approved
	for _, c := range contents {
		updated, err := repo.FindByID(context.Background(), c.ID)
		require.NoError(t, err)
		assert.Equal(t, entity.ContentStatusApproved, updated.Status)
		assert.NotNil(t, updated.ModerationID)
	}

	// Test idempotency: trying to moderate the same content again should fail
	_, err := modSvc.ModerateContent(context.Background(), contents[0].ID)
	assert.Error(t, err)
	assert.Error(t, err)
	assert.True(t,
		strings.Contains(err.Error(), "already being processed") ||
			strings.Contains(err.Error(), "already moderated"),
		"expected idempotency or already moderated error")

	// Cancel context to shutdown pool gracefully
	cancel()
	wg.Wait()
}

func TestWorkerPool_RateLimiting(t *testing.T) {
	repo := mock.NewMockContentRepository()
	aiClient := mock.NewMockAIClient()
	aiClient.SetAsApproving()
	aiClient.SetDelay(10) // small delay to make processing measurable
	cache := mock.NewMockCacheStore()
	broker := mock.NewMockMessageBroker()

	modSvc := service.NewModerationService(repo, aiClient, cache, broker)

	// Prepare 4 jobs (burst is 2, then must wait)
	const numJobs = 4
	contents := make([]*entity.Content, numJobs)
	for i := 0; i < numJobs; i++ {
		c, _ := entity.NewContent("user_rate", entity.ContentTypeText, "rate test")
		c.ID = "rate-" + string(rune('0'+i))
		repo.Save(context.Background(), c)
		broker.PublishModerationJob(context.Background(), c)
		contents[i] = c
	}

	cfg := worker.Config{
		WorkerCount: 1,
		RateLimit:   2, // 2 jobs per second
		RateBurst:   1, // only 1 immediate
		JobTimeout:  5 * time.Second,
	}
	pool := worker.NewPool(cfg, modSvc, broker)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	go pool.Start(ctx)

	// Wait for all to be approved
	deadline := time.Now().Add(5 * time.Second)
	for {
		approved := 0
		for _, c := range contents {
			fetched, _ := repo.FindByID(context.Background(), c.ID)
			if fetched != nil && fetched.Status == entity.ContentStatusApproved {
				approved++
			}
		}
		if approved == numJobs {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timeout")
		}
		time.Sleep(50 * time.Millisecond)
	}
	elapsed := time.Since(start)

	// With rate limit 2/s, 4 jobs should take more than 1.5 seconds (first job immediate, then one per 0.5s)
	// Actually with burst 1: first immediate, second after 0.5s, third after 1.0s, fourth after 1.5s = total ~1.5s
	// Add processing delay 10ms per job negligible.
	assert.GreaterOrEqual(t, elapsed, 1500*time.Millisecond, "Rate limiting should delay processing beyond burst")

	cancel()
}
