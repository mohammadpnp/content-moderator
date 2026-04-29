package inbound

import (
	"context"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
)

type NotificationService interface {
	NotifyUser(ctx context.Context, userID, contentID string, notifType entity.NotificationType, message string) error

	GetUserNotifications(ctx context.Context, userID string, limit, offset int) ([]*entity.Notification, error)
}
