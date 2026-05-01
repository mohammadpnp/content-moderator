package service

import (
	"context"
	"testing"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/test/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupContentService() (*ContentServiceImpl, *mock.MockContentRepository, *mock.MockMessageBroker) {
	repo := mock.NewMockContentRepository()
	broker := mock.NewMockMessageBroker()
	service := NewContentService(repo, broker)
	return service, repo, broker
}

func TestContentService_CreateContent(t *testing.T) {
	ctx := context.Background()
	service, repo, broker := setupContentService()

	t.Run("successful creation", func(t *testing.T) {
		content, err := service.CreateContent(ctx, "user-1", entity.ContentTypeText, "Hello, this is a test message")

		require.NoError(t, err)
		assert.NotNil(t, content)
		assert.NotEmpty(t, content.ID)
		assert.Equal(t, "user-1", content.UserID)
		assert.Equal(t, entity.ContentTypeText, content.Type)
		assert.Equal(t, entity.ContentStatusPending, content.Status)
		assert.Equal(t, "Hello, this is a test message", content.Body)

		assert.Equal(t, 1, repo.GetContentCount())

		assert.Equal(t, 1, broker.GetPublishedJobCount())
	})

	t.Run("successful image creation", func(t *testing.T) {
		content, err := service.CreateContent(ctx, "user-2", entity.ContentTypeImage, "https://example.com/image.jpg")

		require.NoError(t, err)
		assert.Equal(t, entity.ContentTypeImage, content.Type)
		assert.Equal(t, "https://example.com/image.jpg", content.Body)
	})

	t.Run("invalid content type", func(t *testing.T) {
		content, err := service.CreateContent(ctx, "user-1", entity.ContentType("invalid"), "test")

		assert.Error(t, err)
		assert.Nil(t, content)
		assert.Contains(t, err.Error(), "invalid content type")
	})

	t.Run("empty body", func(t *testing.T) {
		content, err := service.CreateContent(ctx, "user-1", entity.ContentTypeText, "")

		assert.Error(t, err)
		assert.Nil(t, content)
		assert.Contains(t, err.Error(), "body cannot be empty")
	})

	t.Run("body too long", func(t *testing.T) {
		longBody := make([]byte, 10001)
		for i := range longBody {
			longBody[i] = 'a'
		}

		content, err := service.CreateContent(ctx, "user-1", entity.ContentTypeText, string(longBody))

		assert.Error(t, err)
		assert.Nil(t, content)
		assert.Contains(t, err.Error(), "exceeds maximum length")
	})

	t.Run("repository error", func(t *testing.T) {
		repo.ShouldError = true
		repo.ErrorMsg = "database connection failed"

		content, err := service.CreateContent(ctx, "user-1", entity.ContentTypeText, "test")

		assert.Error(t, err)
		assert.Nil(t, content)
		assert.Contains(t, err.Error(), "failed to save content")

		repo.ShouldError = false
	})

	t.Run("broker error does not fail request", func(t *testing.T) {
		broker.ShouldError = true
		broker.ErrorMsg = "nats connection failed"

		content, err := service.CreateContent(ctx, "user-1", entity.ContentTypeText, "test with broker error")

		require.NoError(t, err)
		assert.NotNil(t, content)
		assert.Equal(t, entity.ContentStatusPending, content.Status)

		broker.ShouldError = false
	})
}

func TestContentService_GetContent(t *testing.T) {
	ctx := context.Background()
	service, repo, _ := setupContentService()

	content := &entity.Content{
		ID:     "test-id-1",
		UserID: "user-1",
		Type:   entity.ContentTypeText,
		Body:   "test content",
		Status: entity.ContentStatusPending,
	}
	err := repo.Save(ctx, content)
	require.NoError(t, err)

	t.Run("successful retrieval", func(t *testing.T) {
		result, err := service.GetContent(ctx, "test-id-1")

		require.NoError(t, err)
		assert.Equal(t, "test-id-1", result.ID)
		assert.Equal(t, "user-1", result.UserID)
		assert.Equal(t, "test content", result.Body)
	})

	t.Run("empty id", func(t *testing.T) {
		result, err := service.GetContent(ctx, "")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "content ID cannot be empty")
	})

	t.Run("content not found", func(t *testing.T) {
		result, err := service.GetContent(ctx, "non-existent-id")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to find content")
	})
}

func TestContentService_ListUserContents(t *testing.T) {
	ctx := context.Background()
	service, repo, _ := setupContentService()

	for i := 0; i < 5; i++ {
		content := &entity.Content{
			ID:     "user-1-content-" + string(rune('a'+i)),
			UserID: "user-1",
			Type:   entity.ContentTypeText,
			Body:   "test content",
			Status: entity.ContentStatusPending,
		}
		err := repo.Save(ctx, content)
		require.NoError(t, err)
	}

	content := &entity.Content{
		ID:     "user-2-content-1",
		UserID: "user-2",
		Type:   entity.ContentTypeText,
		Body:   "other user content",
		Status: entity.ContentStatusPending,
	}
	err := repo.Save(ctx, content)
	require.NoError(t, err)

	t.Run("list with default pagination", func(t *testing.T) {
		results, err := service.ListUserContents(ctx, "user-1", 0, 0)

		require.NoError(t, err)
		assert.Len(t, results, 5)
	})

	t.Run("list with custom limit", func(t *testing.T) {
		results, err := service.ListUserContents(ctx, "user-1", 2, 0)

		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("list with offset", func(t *testing.T) {
		results, err := service.ListUserContents(ctx, "user-1", 2, 2)

		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("empty user id", func(t *testing.T) {
		results, err := service.ListUserContents(ctx, "", 10, 0)

		assert.Error(t, err)
		assert.Nil(t, results)
		assert.Contains(t, err.Error(), "user ID cannot be empty")
	})

	t.Run("user with no content", func(t *testing.T) {
		results, err := service.ListUserContents(ctx, "non-existent-user", 10, 0)

		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestContentService_DeleteContent(t *testing.T) {
	ctx := context.Background()
	service, repo, _ := setupContentService()

	content := &entity.Content{
		ID:     "to-delete",
		UserID: "user-1",
		Type:   entity.ContentTypeText,
		Body:   "delete me",
		Status: entity.ContentStatusPending,
	}
	err := repo.Save(ctx, content)
	require.NoError(t, err)

	t.Run("successful deletion", func(t *testing.T) {
		err := service.DeleteContent(ctx, "to-delete")

		require.NoError(t, err)
		assert.Equal(t, 0, repo.GetContentCount())
	})

	t.Run("empty id", func(t *testing.T) {
		err := service.DeleteContent(ctx, "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "content ID cannot be empty")
	})

	t.Run("delete non-existent", func(t *testing.T) {
		err := service.DeleteContent(ctx, "non-existent")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "content not found")
	})
}
