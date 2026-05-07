package outbound

import (
	"context"
)

type RealtimeBroadcaster interface {
	// Publish sends a message to a channel (e.g., Redis channel).
	Publish(ctx context.Context, channel string, message []byte) error

	// Subscribe listens to a channel and calls handler for each message.
	Subscribe(ctx context.Context, channel string, handler func(message []byte)) error
}