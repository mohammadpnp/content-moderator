package service

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/outbound"
)

type NotificationServiceImpl struct {
	messageBroker outbound.MessageBroker
}

func NewNotificationService(messageBroker outbound.MessageBroker) *NotificationServiceImpl {
	return &NotificationServiceImpl{
		messageBroker: messageBroker,
	}
}

func (s *NotificationServiceImpl) NotifyUser(ctx context.Context, userID, contentID string, notifType entity.NotificationType, message string) error {
	notification, err := entity.NewNotification(userID, contentID, notifType, message)
	if err != nil {
		return fmt.Errorf("failed to create notification entity: %w", err)
	}

	notification.ID = uuid.New().String()

	if err := s.messageBroker.PublishNotification(ctx, notification); err != nil {
		return fmt.Errorf("failed to publish notification: %w", err)
	}

	log.Printf("Notification sent to user %s for content %s: %s", userID, contentID, notification.Type)
	return nil
}

func (s *NotificationServiceImpl) GetUserNotifications(ctx context.Context, userID string, limit, offset int) ([]*entity.Notification, error) {
	if userID == "" {
		return nil, errors.New("user ID cannot be empty")
	}

	// TODO: Implement notification storage and retrieval
	// For now, return a helpful error message
	return nil, errors.New("notification storage not implemented yet - will be available in Phase 1")
}
