package outbound

import (
	"context"
	"time"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
)

type CacheStore interface {
	GetModerationResult(ctx context.Context, contentID string) (*entity.ModerationResult, error)

	SetModerationResult(ctx context.Context, contentID string, result *entity.ModerationResult, ttl time.Duration) error

	Invalidate(ctx context.Context, key string) error
}
