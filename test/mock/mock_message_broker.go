package mock

import (
	"context"
	"fmt"
	"sync"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/outbound"
)

type MockMessageBroker struct {
	mu sync.RWMutex

	// Channels to simulate message queues
	moderationJobs    chan *entity.Content
	moderationResults chan *entity.ModerationResult
	notifications     chan *entity.Notification

	// Tracking for test assertions
	PublishedJobs          []*entity.Content
	PublishedNotifications []*entity.Notification
	ReceivedResults        []*entity.ModerationResult

	// Error injection
	ShouldError bool
	ErrorMsg    string
}

func NewMockMessageBroker() *MockMessageBroker {
	return &MockMessageBroker{
		moderationJobs:         make(chan *entity.Content, 100),
		moderationResults:      make(chan *entity.ModerationResult, 100),
		notifications:          make(chan *entity.Notification, 100),
		PublishedJobs:          make([]*entity.Content, 0),
		PublishedNotifications: make([]*entity.Notification, 0),
		ReceivedResults:        make([]*entity.ModerationResult, 0),
	}
}

// Verify that MockMessageBroker implements MessageBroker interface
var _ outbound.MessageBroker = (*MockMessageBroker)(nil)

func (m *MockMessageBroker) PublishModerationJob(ctx context.Context, content *entity.Content) error {
	if m.ShouldError {
		return fmt.Errorf("%s", m.ErrorMsg)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.PublishedJobs = append(m.PublishedJobs, content)

	select {
	case m.moderationJobs <- content:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (m *MockMessageBroker) SubscribeModerationResults(ctx context.Context, handler func(result *entity.ModerationResult) error) error {
	if m.ShouldError {
		return fmt.Errorf("%s", m.ErrorMsg)
	}

	// In a real implementation, this would be a persistent subscription
	// For mock, we just process any results that are published
	go func() {
		for {
			select {
			case result := <-m.moderationResults:
				m.mu.Lock()
				m.ReceivedResults = append(m.ReceivedResults, result)
				m.mu.Unlock()

				if err := handler(result); err != nil {
					// In production, would log or handle error
					fmt.Printf("Error in result handler: %v\n", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (m *MockMessageBroker) PublishNotification(ctx context.Context, notification *entity.Notification) error {
	if m.ShouldError {
		return fmt.Errorf("%s", m.ErrorMsg)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.PublishedNotifications = append(m.PublishedNotifications, notification)

	select {
	case m.notifications <- notification:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (m *MockMessageBroker) GetPublishedJobCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.PublishedJobs)
}

func (m *MockMessageBroker) GetPublishedNotificationCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.PublishedNotifications)
}

func (m *MockMessageBroker) SimulateResult(result *entity.ModerationResult) {
	m.moderationResults <- result
}

func (m *MockMessageBroker) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PublishedJobs = make([]*entity.Content, 0)
	m.PublishedNotifications = make([]*entity.Notification, 0)
	m.ReceivedResults = make([]*entity.ModerationResult, 0)
	m.ShouldError = false
}

func (m *MockMessageBroker) PublishModerationResult(ctx context.Context, result *entity.ModerationResult) error {
	if m.ShouldError {
		return fmt.Errorf("%s", m.ErrorMsg)
	}
	select {
	case m.moderationResults <- result:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
