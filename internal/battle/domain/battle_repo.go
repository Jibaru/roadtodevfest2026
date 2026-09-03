package domain

import "context"

// BattleRepository stores the single current battle of the show.
// Implementations live in infra/persistence and must return domain errors.
type BattleRepository interface {
	// Save persists the battle as the current one.
	Save(ctx context.Context, b *Battle) error
	// Current returns the current battle or ErrBattleNotFound.
	Current(ctx context.Context) (*Battle, error)
	// Clear removes the current battle (rehearsal reset).
	Clear(ctx context.Context) error
}

// VerseCache provides pre-written emergency verses used when the
// LLM is unavailable mid-show. Read-only.
type VerseCache interface {
	// Emergency returns a fallback verse for a battler and topic,
	// or ErrVerseNotCached.
	Emergency(battler Battler, topic string) (string, error)
}
