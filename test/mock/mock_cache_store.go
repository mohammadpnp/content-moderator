package mock

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/outbound"
)

type cacheItem struct {
	Result    *entity.ModerationResult
	ExpiresAt time.Time
}

type MockCacheStore struct {
	mu    sync.RWMutex
	items map[string]*cacheItem

	ShouldError bool
	ErrorMsg    string
}

func NewMockCacheStore() *MockCacheStore {
	return &MockCacheStore{
		items: make(map[string]*cacheItem),
	}
}

// Verify that MockCacheStore implements CacheStore interface
var _ outbound.CacheStore = (*MockCacheStore)(nil)

func (m *MockCacheStore) GetModerationResult(ctx context.Context, contentID string) (*entity.ModerationResult, error) {
	if m.ShouldError {
		return nil, fmt.Errorf("%s", m.ErrorMsg)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	item, exists := m.items[contentID]
	if !exists {
		return nil, fmt.Errorf("cache miss for content: %s", contentID)
	}

	if time.Now().After(item.ExpiresAt) {
		delete(m.items, contentID)
		return nil, fmt.Errorf("cache expired for content: %s", contentID)
	}

	// Return a copy to avoid external modifications
	resultCopy := *item.Result
	return &resultCopy, nil
}

func (m *MockCacheStore) SetModerationResult(ctx context.Context, contentID string, result *entity.ModerationResult, ttl time.Duration) error {
	if m.ShouldError {
		return fmt.Errorf("%s", m.ErrorMsg)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Store a copy to avoid external modifications
	resultCopy := *result
	m.items[contentID] = &cacheItem{
		Result:    &resultCopy,
		ExpiresAt: time.Now().Add(ttl),
	}

	return nil
}

func (m *MockCacheStore) Invalidate(ctx context.Context, key string) error {
	if m.ShouldError {
		return fmt.Errorf("%s", m.ErrorMsg)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.items, key)
	return nil
}

func (m *MockCacheStore) GetCacheSize() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.items)
}

func (m *MockCacheStore) IsExpired(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, exists := m.items[key]
	if !exists {
		return true
	}

	return time.Now().After(item.ExpiresAt)
}

func (m *MockCacheStore) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = make(map[string]*cacheItem)
	m.ShouldError = false
}
