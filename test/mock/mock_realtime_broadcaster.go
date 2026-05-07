package mock

import (
	"context"
	"fmt"
	"sync"

	"github.com/mohammadpnp/content-moderator/internal/domain/port/outbound"
)

type MockRealtimeBroadcaster struct {
	mu          sync.Mutex
	messages    map[string][][]byte
	handlers    map[string]func([]byte)
	ShouldError bool
	ErrorMsg    string
}

func NewMockRealtimeBroadcaster() *MockRealtimeBroadcaster {
	return &MockRealtimeBroadcaster{
		messages: make(map[string][][]byte),
		handlers: make(map[string]func([]byte)),
	}
}

var _ outbound.RealtimeBroadcaster = (*MockRealtimeBroadcaster)(nil)

func (m *MockRealtimeBroadcaster) Publish(ctx context.Context, channel string, message []byte) error {
	if m.ShouldError {
		return fmt.Errorf(m.ErrorMsg)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages[channel] = append(m.messages[channel], message)

	if handler, ok := m.handlers[channel]; ok {
		handler(message)
	}
	return nil
}

func (m *MockRealtimeBroadcaster) Subscribe(ctx context.Context, channel string, handler func(message []byte)) error {
	if m.ShouldError {
		return fmt.Errorf(m.ErrorMsg)
	}
	m.mu.Lock()
	m.handlers[channel] = handler
	m.mu.Unlock()
	return nil
}

func (m *MockRealtimeBroadcaster) GetPublished(channel string) [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	msgs := m.messages[channel]
	out := make([][]byte, len(msgs))
	copy(out, msgs)
	return out
}
