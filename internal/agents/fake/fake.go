// Package fake provides no-API rehearsal doubles for the crew and the
// performer. Run the whole show offline with FAKE_AGENTS=1.
package fake

import (
	"context"
	"fmt"
	"time"

	"github.com/jibaru/agentarena/internal/battle/domain"
)

// Crew fakes both battlers and the judge using the emergency verse cache.
type Crew struct {
	Cache domain.VerseCache
	// Delay simulates model latency so the "writing" phase is visible.
	Delay time.Duration
}

func (c *Crew) WriteVerse(ctx context.Context, battler domain.Battler, topic string, _ []domain.RoundState) (string, error) {
	select {
	case <-time.After(c.Delay):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return c.Cache.Emergency(battler, topic)
}

func (c *Crew) Commentary(_ context.Context, state domain.BattleState) (string, error) {
	round := state.Rounds[len(state.Rounds)-1]
	return fmt.Sprintf("Round %d is in the books! The crowd went %d-%d — what a battle, and we're just getting warmed up!",
		round.Number,
		round.VoteCounts[domain.BattlerBlue],
		round.VoteCounts[domain.BattlerRed],
	), nil
}

// SilentPerformer always fails, which drives the text-only fallback
// path: the stage shows lyrics and the presenter performs them.
type SilentPerformer struct{}

func (SilentPerformer) Synthesize(context.Context, domain.Battler, string) ([]byte, error) {
	return nil, fmt.Errorf("fake mode: no TTS")
}
