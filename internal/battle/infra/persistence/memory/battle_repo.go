package memory

import (
	"context"
	"sync"

	"github.com/jibaru/agentarena/internal/battle/domain"
)

// BattleRepository keeps the current battle in memory.
// It is the source of truth during the live show.
type BattleRepository struct {
	mu      sync.RWMutex
	current *domain.Battle
}

func NewBattleRepository() *BattleRepository {
	return &BattleRepository{}
}

func (r *BattleRepository) Save(_ context.Context, b *domain.Battle) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.current = b
	return nil
}

func (r *BattleRepository) Current(_ context.Context) (*domain.Battle, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.current == nil {
		return nil, domain.ErrBattleNotFound
	}
	return r.current, nil
}

func (r *BattleRepository) Clear(_ context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.current = nil
	return nil
}
