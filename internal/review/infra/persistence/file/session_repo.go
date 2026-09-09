package file

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/jibaru/agentarena/internal/review/domain"
)

// SessionRepository persists the current session as a JSON snapshot on
// disk. Used for local rehearsals: replay a session across restarts.
// Implements the same interface as the memory repo — swapping storage
// is one constructor line in main.
type SessionRepository struct {
	path string
}

func NewSessionRepository(dir string) (*SessionRepository, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &SessionRepository{path: filepath.Join(dir, "session.json")}, nil
}

func (r *SessionRepository) Save(_ context.Context, s *domain.Session) error {
	data, err := json.MarshalIndent(s.Snapshot(), "", "  ")
	if err != nil {
		return err
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, r.path)
}

func (r *SessionRepository) Current(_ context.Context) (*domain.Session, error) {
	data, err := os.ReadFile(r.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, domain.ErrSessionNotFound
		}
		return nil, err
	}
	var st domain.SessionState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return domain.RestoreSession(st), nil
}

func (r *SessionRepository) Clear(_ context.Context) error {
	err := os.Remove(r.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
