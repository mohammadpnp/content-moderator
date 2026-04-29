package outbound

import (
	"context"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
)

type AIClient interface {
	ModerateText(ctx context.Context, text string) (*entity.ModerationResult, error)

	ModerateImage(ctx context.Context, imageURL string) (*entity.ModerationResult, error)

	IsHealthy(ctx context.Context) bool
}
