// Package adapters provides concrete implementations of the ports interfaces.
package adapters

import (
	"context"
	"sync"

	"github.com/subzone/Agentctl/internal/ports"
)

// MemoryStore is an in-memory StateStore. Sessions survive for the
// lifetime of the process but are lost on exit. This is the default
// when no persistent store is configured.
type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*ports.SessionData
}

// NewMemoryStore constructs an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{sessions: make(map[string]*ports.SessionData)}
}

func (m *MemoryStore) SaveSession(_ context.Context, id string, data *ports.SessionData) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[id] = data
	return nil
}

func (m *MemoryStore) LoadSession(_ context.Context, id string) (*ports.SessionData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.sessions[id]
	if !ok {
		return nil, &ports.ErrNotFound{ID: id}
	}
	return d, nil
}

func (m *MemoryStore) ListSessions(_ context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	return ids, nil
}

func (m *MemoryStore) DeleteSession(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
	return nil
}
