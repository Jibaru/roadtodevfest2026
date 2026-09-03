package service

import (
	"context"

	"github.com/jibaru/agentarena/internal/battle/domain"
)

// VerseWriter produces a battler's verse for a topic. Implemented by
// the ADK agents package; the service only knows this port.
type VerseWriter interface {
	WriteVerse(ctx context.Context, battler domain.Battler, topic string, history []domain.RoundState) (string, error)
}

// Judge produces commentary after a round's votes are in.
type Judge interface {
	Commentary(ctx context.Context, state domain.BattleState) (string, error)
}

// Performer turns a verse into audio (WAV bytes) with the battler's voice.
type Performer interface {
	Synthesize(ctx context.Context, battler domain.Battler, verse string) ([]byte, error)
}

// Broadcaster fans events out to connected clients. Implemented by
// the realtime hub.
type Broadcaster interface {
	ToAudience(event Event)
	ToStage(event Event)
}
