package memory

import (
	"context"
	"sync"

	"github.com/jibaru/agentarena/internal/review/domain"
)

// SessionRepository keeps the current session in memory.
// It is the source of truth during the live show.
type SessionRepository struct {
	mu      sync.RWMutex
	current *domain.Session
}

func NewSessionRepository() *SessionRepository {
	return &SessionRepository{}
}

func (r *SessionRepository) Save(_ context.Context, s *domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.current = s
	return nil
}

func (r *SessionRepository) Current(_ context.Context) (*domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.current == nil {
		return nil, domain.ErrSessionNotFound
	}
	return r.current, nil
}

func (r *SessionRepository) Clear(_ context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.current = nil
	return nil
}
