// internal/domain/port/inbound/moderation_service.go
package inbound

import (
	"context"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
)

type ModerationService interface {
	ModerateContent(ctx context.Context, contentID string) (*entity.ModerationResult, error)

	HandleModerationResult(ctx context.Context, result *entity.ModerationResult) error

	GetModerationResult(ctx context.Context, contentID string) (*entity.ModerationResult, error)
}
