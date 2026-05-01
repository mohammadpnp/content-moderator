package service

import (
	"context"
	"testing"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/test/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupNotificationService() (*NotificationServiceImpl, *mock.MockMessageBroker) {
	broker := mock.NewMockMessageBroker()
	service := NewNotificationService(broker)
	return service, broker
}

func TestNotificationService_NotifyUser(t *testing.T) {
	ctx := context.Background()
	service, broker := setupNotificationService()

	t.Run("successful approved notification", func(t *testing.T) {
		err := service.NotifyUser(
			ctx,
			"user-1",
			"content-1",
			entity.NotificationApproved,
			"Your content has been approved",
		)

		require.NoError(t, err)
		assert.Equal(t, 1, broker.GetPublishedNotificationCount())
	})

	t.Run("successful rejected notification", func(t *testing.T) {
		err := service.NotifyUser(
			ctx,
			"user-2",
			"content-2",
			entity.NotificationRejected,
			"Your content has been rejected",
		)

		require.NoError(t, err)
		assert.Equal(t, 2, broker.GetPublishedNotificationCount())
	})

	t.Run("empty user id", func(t *testing.T) {
		err := service.NotifyUser(
			ctx,
			"",
			"content-1",
			entity.NotificationApproved,
			"test message",
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user ID cannot be empty")
	})

	t.Run("empty content id", func(t *testing.T) {
		err := service.NotifyUser(
			ctx,
			"user-1",
			"",
			entity.NotificationApproved,
			"test message",
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "content ID cannot be empty")
	})

	t.Run("invalid notification type", func(t *testing.T) {
		err := service.NotifyUser(
			ctx,
			"user-1",
			"content-1",
			entity.NotificationType("invalid"),
			"test message",
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid notification type")
	})

	t.Run("empty message", func(t *testing.T) {
		err := service.NotifyUser(
			ctx,
			"user-1",
			"content-1",
			entity.NotificationApproved,
			"",
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "message cannot be empty")
	})

	t.Run("broker error", func(t *testing.T) {
		broker.ShouldError = true
		broker.ErrorMsg = "connection failed"

		err := service.NotifyUser(
			ctx,
			"user-1",
			"content-1",
			entity.NotificationApproved,
			"test message",
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to publish notification")

		// Reset
		broker.ShouldError = false
	})
}

func TestNotificationService_GetUserNotifications(t *testing.T) {
	ctx := context.Background()
	service, _ := setupNotificationService()

	t.Run("currently returns not implemented", func(t *testing.T) {
		results, err := service.GetUserNotifications(ctx, "user-1", 10, 0)

		assert.Error(t, err)
		assert.Nil(t, results)
		assert.Contains(t, err.Error(), "not implemented")
	})

	t.Run("empty user id", func(t *testing.T) {
		results, err := service.GetUserNotifications(ctx, "", 10, 0)

		assert.Error(t, err)
		assert.Nil(t, results)
		assert.Contains(t, err.Error(), "user ID cannot be empty")
	})
}
