package mock

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/outbound"
)

type MockAIClient struct {
	ShouldApprove   bool
	ConfidenceScore float64
	Categories      []entity.ModerationCategory
	ModelName       string

	ShouldError bool
	ErrorMsg    string

	// Delay simulation (in milliseconds)
	DelayMs int64

	IsServiceHealthy bool
}

func NewMockAIClient() *MockAIClient {
	return &MockAIClient{
		ShouldApprove:    true,
		ConfidenceScore:  0.95,
		Categories:       []entity.ModerationCategory{},
		ModelName:        "mock-bert-classifier",
		DelayMs:          10, // Simulate 10ms processing time
		IsServiceHealthy: true,
	}
}

// Verify that MockAIClient implements AIClient interface
var _ outbound.AIClient = (*MockAIClient)(nil)

func (m *MockAIClient) ModerateText(ctx context.Context, text string) (*entity.ModerationResult, error) {
	if m.ShouldError {
		return nil, fmt.Errorf("%s", m.ErrorMsg)
	}

	select {
	case <-time.After(time.Duration(m.DelayMs) * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	result, err := entity.NewModerationResult(
		"", // contentID will be set by the caller
		m.ShouldApprove,
		m.ConfidenceScore,
		m.Categories,
		m.ModelName,
		m.DelayMs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create moderation result: %w", err)
	}

	result.ID = fmt.Sprintf("mock-result-%d", rand.Intn(10000))

	return result, nil
}

func (m *MockAIClient) ModerateImage(ctx context.Context, imageURL string) (*entity.ModerationResult, error) {
	if m.ShouldError {
		return nil, fmt.Errorf("%s", m.ErrorMsg)
	}

	// Simulate processing delay (images take longer)
	imageDelay := m.DelayMs * 2
	select {
	case <-time.After(time.Duration(imageDelay) * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	result, err := entity.NewModerationResult(
		"", // contentID will be set by the caller
		m.ShouldApprove,
		m.ConfidenceScore,
		m.Categories,
		"mock-vit-classifier", // Different model for images
		imageDelay,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create moderation result: %w", err)
	}

	result.ID = fmt.Sprintf("mock-result-%d", rand.Intn(10000))

	return result, nil
}

func (m *MockAIClient) IsHealthy(ctx context.Context) bool {
	if m.ShouldError {
		return false
	}
	return m.IsServiceHealthy
}

func (m *MockAIClient) SetAsApproving() {
	m.ShouldApprove = true
	m.ConfidenceScore = 0.95
	m.Categories = []entity.ModerationCategory{}
}

func (m *MockAIClient) SetAsRejecting(categories ...entity.ModerationCategory) {
	m.ShouldApprove = false
	m.ConfidenceScore = 0.85
	if len(categories) == 0 {
		m.Categories = []entity.ModerationCategory{entity.CategorySpam}
	} else {
		m.Categories = categories
	}
}

func (m *MockAIClient) SetAsUnhealthy() {
	m.IsServiceHealthy = false
}

func (m *MockAIClient) SetDelay(delayMs int64) {
	m.DelayMs = delayMs
}
