package outbound

import (
	"context"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
)

type ContentRepository interface {
	Save(ctx context.Context, content *entity.Content) error
	
	FindByID(ctx context.Context, id string) (*entity.Content, error)
	
	FindByUserID(ctx context.Context, userID string, limit, offset int) ([]*entity.Content, error)
	
	UpdateStatus(ctx context.Context, id string, status entity.ContentStatus, moderationID *string) error
	
	Delete(ctx context.Context, id string) error
}