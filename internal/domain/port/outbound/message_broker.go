package outbound

import (
	"context"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
)

type MessageBroker interface {
	PublishModerationJob(ctx context.Context, content *entity.Content) error

	SubscribeModerationResults(ctx context.Context, handler func(result *entity.ModerationResult) error) error

	PublishNotification(ctx context.Context, notification *entity.Notification) error
}
