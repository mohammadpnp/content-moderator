package inbound

import (
	"context"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
)

type ContentService interface {
	CreateContent(ctx context.Context, userID string, contentType entity.ContentType, body string) (*entity.Content, error)

	GetContent(ctx context.Context, id string) (*entity.Content, error)

	ListUserContents(ctx context.Context, userID string, limit, offset int) ([]*entity.Content, error)

	DeleteContent(ctx context.Context, id string) error
}
