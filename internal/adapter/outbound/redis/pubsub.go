package redis

import (
	"context"

	"github.com/mohammadpnp/content-moderator/internal/domain/port/outbound"
	"github.com/redis/go-redis/v9"
)

type PubSubAdapter struct {
	client *redis.Client
}

func NewPubSubAdapter(client *redis.Client) *PubSubAdapter {
	return &PubSubAdapter{client: client}
}

// Compile-time check
var _ outbound.RealtimeBroadcaster = (*PubSubAdapter)(nil)

func (p *PubSubAdapter) Publish(ctx context.Context, channel string, message []byte) error {
	return p.client.Publish(ctx, channel, message).Err()
}

func (p *PubSubAdapter) Subscribe(ctx context.Context, channel string, handler func(message []byte)) error {
	sub := p.client.Subscribe(ctx, channel)
	go func() {
		defer sub.Close()
		for msg := range sub.Channel() {
			handler([]byte(msg.Payload))
		}
	}()
	return nil
}
