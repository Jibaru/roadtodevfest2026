package file

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/jibaru/rapbattle/internal/battle/domain"
)

// BattleRepository persists the current battle as a JSON snapshot on disk.
// Used for local rehearsals: replay a battle across restarts without
// burning tokens. Implements the same interface as the memory repo —
// swapping storage is one constructor line in main.
type BattleRepository struct {
	path string
}

func NewBattleRepository(dir string) (*BattleRepository, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &BattleRepository{path: filepath.Join(dir, "battle.json")}, nil
}

func (r *BattleRepository) Save(_ context.Context, b *domain.Battle) error {
	data, err := json.MarshalIndent(b.Snapshot(), "", "  ")
	if err != nil {
		return err
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, r.path)
}

func (r *BattleRepository) Current(_ context.Context) (*domain.Battle, error) {
	data, err := os.ReadFile(r.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, domain.ErrBattleNotFound
		}
		return nil, err
	}
	var s domain.BattleState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return domain.RestoreBattle(s), nil
}

func (r *BattleRepository) Clear(_ context.Context) error {
	err := os.Remove(r.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
