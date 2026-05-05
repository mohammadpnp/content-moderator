package triton

import (
	"context"
	"time"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/outbound"
	"github.com/sony/gobreaker"
)

// CircuitBreakerAIClient wraps an AIClient with a circuit breaker.
type CircuitBreakerAIClient struct {
	client outbound.AIClient
	cb     *gobreaker.CircuitBreaker
}

// NewCircuitBreakerAIClient creates a circuit breaker protected AI client.
func NewCircuitBreakerAIClient(client outbound.AIClient) *CircuitBreakerAIClient {
	settings := gobreaker.Settings{
		Name: "triton-ai",
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Trip after 5 failures in the last 60 seconds
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 5 && failureRatio >= 0.6
		},
		MaxRequests: 3,                // number of requests to pass in half-open state
		Interval:    60 * time.Second, // period of time for counters to reset
		Timeout:     30 * time.Second, // time to stay open before going half-open
	}
	return &CircuitBreakerAIClient{
		client: client,
		cb:     gobreaker.NewCircuitBreaker(settings),
	}
}

func (c *CircuitBreakerAIClient) ModerateText(ctx context.Context, text string) (*entity.ModerationResult, error) {
	result, err := c.cb.Execute(func() (interface{}, error) {
		return c.client.ModerateText(ctx, text)
	})
	if err != nil {
		return nil, err
	}
	return result.(*entity.ModerationResult), nil
}

func (c *CircuitBreakerAIClient) ModerateImage(ctx context.Context, imageURL string) (*entity.ModerationResult, error) {
	result, err := c.cb.Execute(func() (interface{}, error) {
		return c.client.ModerateImage(ctx, imageURL)
	})
	if err != nil {
		return nil, err
	}
	return result.(*entity.ModerationResult), nil
}

func (c *CircuitBreakerAIClient) IsHealthy(ctx context.Context) bool {
	// Health check bypasses the circuit breaker? Or we can also protect it.
	// We'll simply delegate directly.
	// In a more sophisticated setup, we might want the breaker's state to reflect health.
	// But for now, keep it separate.
	return c.client.IsHealthy(ctx)
}

var _ outbound.AIClient = (*CircuitBreakerAIClient)(nil)
