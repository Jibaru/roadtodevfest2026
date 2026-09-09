package domain

import "context"

// SessionRepository stores the single current session of the show.
// Implementations live in infra/persistence and must return domain errors.
type SessionRepository interface {
	// Save persists the session as the current one.
	Save(ctx context.Context, s *Session) error
	// Current returns the current session or ErrSessionNotFound.
	Current(ctx context.Context) (*Session, error)
	// Clear removes the current session (rehearsal reset).
	Clear(ctx context.Context) error
}
