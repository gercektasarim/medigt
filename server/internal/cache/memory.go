package cache

import (
	"context"
	"sync"
	"time"
)

// memoryClient is a tiny in-process fallback for local dev without Redis.
// Not safe for multi-node deployments — production must set REDIS_URL.
type memoryClient struct {
	mu   sync.Mutex
	data map[string]memoryEntry
}

type memoryEntry struct {
	value     string
	counter   int64
	expiresAt time.Time
}

func newMemory() *memoryClient {
	return &memoryClient{data: make(map[string]memoryEntry)}
}

func (m *memoryClient) Get(_ context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.data[key]
	if !ok || (!e.expiresAt.IsZero() && time.Now().After(e.expiresAt)) {
		delete(m.data, key)
		return "", ErrMiss
	}
	return e.value, nil
}

func (m *memoryClient) Set(_ context.Context, key, value string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	exp := time.Time{}
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	m.data[key] = memoryEntry{value: value, expiresAt: exp}
	return nil
}

func (m *memoryClient) Del(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *memoryClient) Incr(_ context.Context, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e := m.data[key]
	e.counter++
	m.data[key] = e
	return e.counter, nil
}

func (m *memoryClient) Expire(_ context.Context, key string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.data[key]
	if !ok {
		return nil
	}
	if ttl > 0 {
		e.expiresAt = time.Now().Add(ttl)
	} else {
		e.expiresAt = time.Time{}
	}
	m.data[key] = e
	return nil
}
