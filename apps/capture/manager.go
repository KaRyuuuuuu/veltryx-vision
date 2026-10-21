package capture

import (
	"context"
	"sync"

	"github.com/veltryx/vision/internal/camera"
	"github.com/veltryx/vision/pkg/models"
)

type Manager struct {
	mu       sync.RWMutex
	provider camera.Provider
	sessions map[string]*Session
}

func New(p camera.Provider) *Manager { return &Manager{provider: p, sessions: map[string]*Session{}} }
func (m *Manager) Open(ctx context.Context, c models.Camera) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s := m.sessions[c.Serial]; s != nil {
		return s, nil
	}
	d, e := m.provider.Open(ctx, c)
	if e != nil {
		return nil, e
	}
	s := newSession(d)
	m.sessions[c.Serial] = s
	return s, nil
}
func (m *Manager) Get(serial string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[serial]
}
func (m *Manager) Remove(serial string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s := m.sessions[serial]; s != nil {
		_ = s.Close()
		delete(m.sessions, serial)
	}
}
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.sessions {
		_ = s.Close()
		delete(m.sessions, id)
	}
}
