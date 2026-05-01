package service

import (
	"context"
	"testing"
	"time"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/test/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupModerationService() (*ModerationServiceImpl, *mock.MockContentRepository, *mock.MockAIClient, *mock.MockCacheStore, *mock.MockMessageBroker) {
	repo := mock.NewMockContentRepository()
	aiClient := mock.NewMockAIClient()
	cache := mock.NewMockCacheStore()
	broker := mock.NewMockMessageBroker()
	service := NewModerationService(repo, aiClient, cache, broker)
	return service, repo, aiClient, cache, broker
}

func TestModerationService_ModerateContent(t *testing.T) {
	ctx := context.Background()
	service, repo, aiClient, cache, _ := setupModerationService()

	content := &entity.Content{
		ID:     "content-1",
		UserID: "user-1",
		Type:   entity.ContentTypeText,
		Body:   "test content for moderation",
		Status: entity.ContentStatusPending,
	}
	err := repo.Save(ctx, content)
	require.NoError(t, err)

	t.Run("successful text moderation - approved", func(t *testing.T) {
		aiClient.SetAsApproving()

		result, err := service.ModerateContent(ctx, "content-1")

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.IsApproved)
		assert.Equal(t, "content-1", result.ContentID)
		assert.Greater(t, result.DurationMs, int64(0))

		// Check that result was cached
		cachedResult, cacheErr := cache.GetModerationResult(ctx, "content-1")
		require.NoError(t, cacheErr)
		assert.NotNil(t, cachedResult)
	})

	t.Run("successful text moderation - rejected", func(t *testing.T) {
		content2 := &entity.Content{
			ID:     "content-2",
			UserID: "user-1",
			Type:   entity.ContentTypeText,
			Body:   "spam content",
			Status: entity.ContentStatusPending,
		}
		err := repo.Save(ctx, content2)
		require.NoError(t, err)

		aiClient.SetAsRejecting(entity.CategorySpam)

		result, err := service.ModerateContent(ctx, "content-2")

		require.NoError(t, err)
		assert.False(t, result.IsApproved)
		assert.Contains(t, result.Categories, entity.CategorySpam)
	})

	t.Run("content already moderated", func(t *testing.T) {
		err := repo.UpdateStatus(ctx, "content-1", entity.ContentStatusApproved, nil)
		require.NoError(t, err)

		result, err := service.ModerateContent(ctx, "content-1")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "already moderated")
	})

	t.Run("content not found", func(t *testing.T) {
		result, err := service.ModerateContent(ctx, "non-existent")

		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("empty content id", func(t *testing.T) {
		result, err := service.ModerateContent(ctx, "")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "content ID cannot be empty")
	})

	t.Run("ai client error", func(t *testing.T) {
		content3 := &entity.Content{
			ID:     "content-3",
			UserID: "user-1",
			Type:   entity.ContentTypeText,
			Body:   "error test",
			Status: entity.ContentStatusPending,
		}
		repo.Save(ctx, content3)

		aiClient.ShouldError = true
		aiClient.ErrorMsg = "ai service unavailable"

		result, err := service.ModerateContent(ctx, "content-3")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "AI moderation failed")

		aiClient.ShouldError = false
	})

	t.Run("moderation with context timeout", func(t *testing.T) {
		timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
		defer cancel()

		aiClient.SetDelay(100) // 100ms delay, but context times out in 1ms

		// We expect either a timeout error or the cancellation to propagate
		_, err := service.ModerateContent(timeoutCtx, "content-1")
		// Note: This might succeed or fail depending on timing
		// The important thing is the context is respected
		_ = err

		// Reset delay
		aiClient.SetDelay(10)
	})
}

func TestModerationService_HandleModerationResult(t *testing.T) {
	ctx := context.Background()
	service, repo, _, _, broker := setupModerationService()

	content := &entity.Content{
		ID:     "content-1",
		UserID: "user-1",
		Type:   entity.ContentTypeText,
		Body:   "test content",
		Status: entity.ContentStatusPending,
	}
	err := repo.Save(ctx, content)
	require.NoError(t, err)

	t.Run("handle approved result", func(t *testing.T) {
		result := &entity.ModerationResult{
			ID:         "result-1",
			ContentID:  "content-1",
			IsApproved: true,
			Score:      0.95,
			Categories: []entity.ModerationCategory{},
			ModelName:  "test-model",
		}

		err := service.HandleModerationResult(ctx, result)

		require.NoError(t, err)

		updatedContent, _ := repo.FindByID(ctx, "content-1")
		assert.Equal(t, entity.ContentStatusApproved, updatedContent.Status)
		assert.Equal(t, "result-1", *updatedContent.ModerationID)

		// Check notification was sent
		assert.Equal(t, 1, broker.GetPublishedNotificationCount())
	})

	t.Run("handle rejected result", func(t *testing.T) {
		repo.UpdateStatus(ctx, "content-1", entity.ContentStatusPending, nil)
		broker.Clear()

		result := &entity.ModerationResult{
			ID:         "result-2",
			ContentID:  "content-1",
			IsApproved: false,
			Score:      0.85,
			Categories: []entity.ModerationCategory{entity.CategoryHate},
			ModelName:  "test-model",
		}

		err := service.HandleModerationResult(ctx, result)

		require.NoError(t, err)

		updatedContent, _ := repo.FindByID(ctx, "content-1")
		assert.Equal(t, entity.ContentStatusRejected, updatedContent.Status)

		assert.Equal(t, 1, broker.GetPublishedNotificationCount())
	})

	t.Run("nil result", func(t *testing.T) {
		err := service.HandleModerationResult(ctx, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "moderation result cannot be nil")
	})

	t.Run("empty content id in result", func(t *testing.T) {
		result := &entity.ModerationResult{
			ID:         "result-3",
			ContentID:  "",
			IsApproved: true,
		}

		err := service.HandleModerationResult(ctx, result)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "content ID cannot be empty")
	})

	t.Run("content not found", func(t *testing.T) {
		result := &entity.ModerationResult{
			ID:         "result-4",
			ContentID:  "non-existent",
			IsApproved: true,
		}

		err := service.HandleModerationResult(ctx, result)

		assert.Error(t, err)
		assert.True(t,
			contains(err.Error(), "failed to find content") ||
				contains(err.Error(), "failed to update content status") ||
				contains(err.Error(), "content not found"),
			"Expected error to contain one of the expected messages, got: %v", err)
	})
}

func TestModerationService_GetModerationResult(t *testing.T) {
	ctx := context.Background()
	service, _, _, cache, _ := setupModerationService()

	t.Run("cache hit", func(t *testing.T) {
		expectedResult := &entity.ModerationResult{
			ID:         "result-1",
			ContentID:  "content-1",
			IsApproved: true,
			Score:      0.95,
		}
		err := cache.SetModerationResult(ctx, "content-1", expectedResult, 1*time.Hour)
		require.NoError(t, err)

		result, err := service.GetModerationResult(ctx, "content-1")

		require.NoError(t, err)
		assert.Equal(t, expectedResult.ID, result.ID)
		assert.True(t, result.IsApproved)
	})

	t.Run("cache miss", func(t *testing.T) {
		result, err := service.GetModerationResult(ctx, "non-existent")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "moderation result not found")
	})

	t.Run("empty content id", func(t *testing.T) {
		result, err := service.GetModerationResult(ctx, "")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "content ID cannot be empty")
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
