package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/internal/service"
	"github.com/mohammadpnp/content-moderator/internal/worker"
	"github.com/mohammadpnp/content-moderator/test/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerPool_ProcessesJob(t *testing.T) {
	// Setup mock dependencies
	repo := mock.NewMockContentRepository()
	aiClient := mock.NewMockAIClient()
	cache := mock.NewMockCacheStore()
	broker := mock.NewMockMessageBroker()

	// Svc with mock AI that approves everything
	aiClient.SetAsApproving()
	modSvc := service.NewModerationService(repo, aiClient, cache, broker)

	// Create a content that is pending
	content, err := entity.NewContent("user1", entity.ContentTypeText, "test message")
	require.NoError(t, err)
	content.ID = "test-content-1"
	err = repo.Save(context.Background(), content)
	require.NoError(t, err)

	// Publish the job manually (simulating what ContentService does)
	err = broker.PublishModerationJob(context.Background(), content)
	require.NoError(t, err)

	// Create worker pool
	pool := worker.NewPool(worker.DefaultConfig(), modSvc, broker)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start pool in a goroutine
	go pool.Start(ctx)

	// Wait for processing (polling DB status)
	var updated *entity.Content
	for i := 0; i < 10; i++ {
		updated, err = repo.FindByID(context.Background(), content.ID)
		if err == nil && updated.Status == entity.ContentStatusApproved {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	assert.Equal(t, entity.ContentStatusApproved, updated.Status)
	assert.NotNil(t, updated.ModerationID)
}
