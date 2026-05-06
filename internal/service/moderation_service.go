package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/outbound"
)

type ModerationServiceImpl struct {
	repo          outbound.ContentRepository
	aiClient      outbound.AIClient
	cacheStore    outbound.CacheStore
	messageBroker outbound.MessageBroker
}

func NewModerationService(
	repo outbound.ContentRepository,
	aiClient outbound.AIClient,
	cacheStore outbound.CacheStore,
	messageBroker outbound.MessageBroker,
) *ModerationServiceImpl {
	return &ModerationServiceImpl{
		repo:          repo,
		aiClient:      aiClient,
		cacheStore:    cacheStore,
		messageBroker: messageBroker,
	}
}

func (s *ModerationServiceImpl) ModerateContent(ctx context.Context, contentID string) (*entity.ModerationResult, error) {
	if contentID == "" {
		return nil, errors.New("content ID cannot be empty")
	}

	content, err := s.repo.FindByID(ctx, contentID)
	if err != nil {
		return nil, fmt.Errorf("failed to find content for moderation: %w", err)
	}

	if !content.IsPending() {
		return nil, fmt.Errorf("content %s is already moderated (status: %s)", contentID, content.Status)
	}

	idempotencyKey := "idempotent:moderate:" + contentID
	acquired, err := s.cacheStore.SetIfNotExists(ctx, idempotencyKey, struct{}{}, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("idempotency check failed: %w", err)
	}
	if !acquired {
		return nil, fmt.Errorf("content %s is already being processed", contentID)
	}

	var result *entity.ModerationResult
	startTime := time.Now()

	switch content.Type {
	case entity.ContentTypeText:
		result, err = s.aiClient.ModerateText(ctx, content.Body)
	case entity.ContentTypeImage:
		result, err = s.aiClient.ModerateImage(ctx, content.Body)
	default:
		return nil, fmt.Errorf("unsupported content type: %s", content.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("AI moderation failed: %w", err)
	}

	// Set metadata
	result.ContentID = contentID
	result.DurationMs = time.Since(startTime).Milliseconds()

	// Cache the result
	cacheTTL := 1 * time.Hour
	if cacheErr := s.cacheStore.SetModerationResult(ctx, contentID, result, cacheTTL); cacheErr != nil {
		log.Printf("WARNING: failed to cache moderation result for content %s: %v", contentID, cacheErr)
	}

	// Publish result to NATS for async processing
	if pubErr := s.messageBroker.PublishModerationResult(ctx, result); pubErr != nil {
		log.Printf("WARNING: failed to publish moderation result to NATS: %v", pubErr)
	}

	return result, nil
}

func (s *ModerationServiceImpl) HandleModerationResult(ctx context.Context, result *entity.ModerationResult) error {
	if result == nil {
		return errors.New("moderation result cannot be nil")
	}

	if result.ContentID == "" {
		return errors.New("moderation result content ID cannot be empty")
	}

	var newStatus entity.ContentStatus
	var notificationType entity.NotificationType
	var notificationMessage string

	if result.IsApproved {
		newStatus = entity.ContentStatusApproved
		notificationType = entity.NotificationApproved
		notificationMessage = fmt.Sprintf("Your content has been approved with confidence score: %.2f", result.Score)
	} else {
		newStatus = entity.ContentStatusRejected
		notificationType = entity.NotificationRejected
		notificationMessage = fmt.Sprintf("Your content has been rejected. Categories: %v, Score: %.2f", result.Categories, result.Score)
	}

	if err := s.repo.UpdateStatus(ctx, result.ContentID, newStatus, &result.ID); err != nil {
		return fmt.Errorf("failed to update content status: %w", err)
	}

	content, err := s.repo.FindByID(ctx, result.ContentID)
	if err != nil {
		return fmt.Errorf("failed to find content for notification: %w", err)
	}

	notification, err := entity.NewNotification(
		content.UserID,
		result.ContentID,
		notificationType,
		notificationMessage,
	)
	if err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}

	if err := s.messageBroker.PublishNotification(ctx, notification); err != nil {
		log.Printf("WARNING: failed to publish notification for content %s: %v", result.ContentID, err)
	}

	return nil
}

func (s *ModerationServiceImpl) GetModerationResult(ctx context.Context, contentID string) (*entity.ModerationResult, error) {
	if contentID == "" {
		return nil, errors.New("content ID cannot be empty")
	}

	result, err := s.cacheStore.GetModerationResult(ctx, contentID)
	if err == nil && result != nil {
		log.Printf("Cache hit for moderation result: %s", contentID)
		return result, nil
	}

	return nil, fmt.Errorf("moderation result not found for content %s", contentID)
}
