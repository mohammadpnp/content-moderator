package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/outbound"
)

type ContentServiceImpl struct {
	repo          outbound.ContentRepository
	messageBroker outbound.MessageBroker
}

func NewContentService(repo outbound.ContentRepository, messageBroker outbound.MessageBroker) *ContentServiceImpl {
	return &ContentServiceImpl{
		repo:          repo,
		messageBroker: messageBroker,
	}
}

func (s *ContentServiceImpl) CreateContent(ctx context.Context, userID string, contentType entity.ContentType, body string) (*entity.Content, error) {
	content, err := entity.NewContent(userID, contentType, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create content entity: %w", err)
	}

	content.ID = uuid.New().String()
	content.CreatedAt = time.Now()
	content.UpdatedAt = time.Now()

	if err := s.repo.Save(ctx, content); err != nil {
		return nil, fmt.Errorf("failed to save content: %w", err)
	}

	if s.messageBroker != nil {
		if err := s.messageBroker.PublishModerationJob(ctx, content); err != nil {
			log.Printf("WARNING: failed to publish moderation job for content %s: %v", content.ID, err)
		}
	}

	return content, nil
}

func (s *ContentServiceImpl) GetContent(ctx context.Context, id string) (*entity.Content, error) {
	if id == "" {
		return nil, errors.New("content ID cannot be empty")
	}

	content, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to find content: %w", err)
	}

	return content, nil
}

func (s *ContentServiceImpl) ListUserContents(ctx context.Context, userID string, limit, offset int) ([]*entity.Content, error) {
	if userID == "" {
		return nil, errors.New("user ID cannot be empty")
	}

	// Set default pagination values
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	contents, err := s.repo.FindByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list contents: %w", err)
	}

	return contents, nil
}

func (s *ContentServiceImpl) DeleteContent(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("content ID cannot be empty")
	}

	_, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("content not found: %w", err)
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete content: %w", err)
	}

	return nil
}
