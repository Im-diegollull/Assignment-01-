package cache

import (
	"context"
	"sync"
)


type Memory struct {
	mu   sync.RWMutex
	data map[string][]byte
}

func NewMemory() *Memory {
	return &Memory{data: make(map[string][]byte)}
}

func (*Memory) Name() string { return "memory" }

func (m *Memory) Get(_ context.Context, key string) ([]byte, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.data[key]
	if !ok {
		return nil, false, nil
	}
	out := make([]byte, len(value))
	copy(out, value)
	return out, true, nil
}

func (m *Memory) Set(_ context.Context, key string, value []byte) error {
	stored := make([]byte, len(value))
	copy(stored, value)
	m.mu.Lock()
	m.data[key] = stored
	m.mu.Unlock()
	return nil
}

func (m *Memory) Del(_ context.Context, keys ...string) error {
	m.mu.Lock()
	for _, key := range keys {
		delete(m.data, key)
	}
	m.mu.Unlock()
	return nil
}

func (m *Memory) Flush(_ context.Context) error {
	m.mu.Lock()
	m.data = make(map[string][]byte)
	m.mu.Unlock()
	return nil
}

// Has es un helper de tests.
func (m *Memory) Has(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.data[key]
	return ok
}
