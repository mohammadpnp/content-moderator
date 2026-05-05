package triton_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mohammadpnp/content-moderator/internal/adapter/outbound/triton"
	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/outbound"
	"github.com/stretchr/testify/assert"
)

type failingAIClient struct{}

func (f *failingAIClient) ModerateText(ctx context.Context, text string) (*entity.ModerationResult, error) {
	return nil, errors.New("ai error")
}
func (f *failingAIClient) ModerateImage(ctx context.Context, url string) (*entity.ModerationResult, error) {
	return nil, errors.New("ai error")
}
func (f *failingAIClient) IsHealthy(ctx context.Context) bool { return false }

var _ outbound.AIClient = (*failingAIClient)(nil)

func TestCircuitBreaker_OpensAfterFailures(t *testing.T) {
	client := triton.NewCircuitBreakerAIClient(&failingAIClient{})

	ctx := context.Background()
	// ۵ تا خطا باعث باز شدن مدار می‌شود
	for i := 0; i < 5; i++ {
		_, err := client.ModerateText(ctx, "test")
		assert.Error(t, err)
	}

	// این درخواست باید فوراً با خطای مدار باز برگردد
	_, err := client.ModerateText(ctx, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker")
}

func TestCircuitBreaker_HealthBypassesBreaker(t *testing.T) {
	client := triton.NewCircuitBreakerAIClient(&failingAIClient{})

	ctx := context.Background()
	// حتی وقتی مدار باز می‌شود، IsHealthy مستقیماً صدا زده می‌شود
	for i := 0; i < 5; i++ {
		client.ModerateText(ctx, "test")
	}
	// مدار باید باز شده باشد
	assert.False(t, client.IsHealthy(ctx)) // چون failing client سالم نیست
}
