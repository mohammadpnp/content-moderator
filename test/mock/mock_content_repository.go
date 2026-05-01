package mock

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/outbound"
)

type MockContentRepository struct {
	mu       sync.RWMutex
	contents map[string]*entity.Content
	// ErrorInjection for testing error scenarios
	ShouldError bool
	ErrorMsg    string
}

func NewMockContentRepository() *MockContentRepository {
	return &MockContentRepository{
		contents:    make(map[string]*entity.Content),
		ShouldError: false,
	}
}

var _ outbound.ContentRepository = (*MockContentRepository)(nil)

func (m *MockContentRepository) Save(ctx context.Context, content *entity.Content) error {
	if m.ShouldError {
		return fmt.Errorf("%s", m.ErrorMsg)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if content.ID == "" {
		content.ID = fmt.Sprintf("mock-content-%d", len(m.contents)+1)
	}

	if content.CreatedAt.IsZero() {
		content.CreatedAt = time.Now()
	}
	if content.UpdatedAt.IsZero() {
		content.UpdatedAt = time.Now()
	}

	contentCopy := *content
	m.contents[content.ID] = &contentCopy

	return nil
}

func (m *MockContentRepository) FindByID(ctx context.Context, id string) (*entity.Content, error) {
	if m.ShouldError {
		return nil, fmt.Errorf("%s", m.ErrorMsg)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	content, exists := m.contents[id]
	if !exists {
		return nil, fmt.Errorf("content not found: %s", id)
	}

	contentCopy := *content
	return &contentCopy, nil
}

func (m *MockContentRepository) FindByUserID(ctx context.Context, userID string, limit, offset int) ([]*entity.Content, error) {
	if m.ShouldError {
		return nil, fmt.Errorf("%s", m.ErrorMsg)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var userContents []*entity.Content

	for _, content := range m.contents {
		if content.UserID == userID {
			contentCopy := *content
			userContents = append(userContents, &contentCopy)
		}
	}

	start := offset
	if start > len(userContents) {
		return []*entity.Content{}, nil
	}

	end := start + limit
	if end > len(userContents) {
		end = len(userContents)
	}

	return userContents[start:end], nil
}

func (m *MockContentRepository) UpdateStatus(ctx context.Context, id string, status entity.ContentStatus, moderationID *string) error {
	if m.ShouldError {
		return fmt.Errorf("%s", m.ErrorMsg)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	content, exists := m.contents[id]
	if !exists {
		return fmt.Errorf("content not found: %s", id)
	}

	content.Status = status
	if moderationID != nil {
		content.ModerationID = moderationID
	}
	content.UpdatedAt = time.Now()

	return nil
}

func (m *MockContentRepository) Delete(ctx context.Context, id string) error {
	if m.ShouldError {
		return fmt.Errorf("%s", m.ErrorMsg)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.contents[id]; !exists {
		return fmt.Errorf("content not found: %s", id)
	}

	delete(m.contents, id)
	return nil
}

func (m *MockContentRepository) GetContentCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.contents)
}

func (m *MockContentRepository) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.contents = make(map[string]*entity.Content)
	m.ShouldError = false
}
