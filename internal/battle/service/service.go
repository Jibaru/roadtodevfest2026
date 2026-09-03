package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jibaru/rapbattle/internal/battle/domain"
)

const (
	verseTimeout = 45 * time.Second
	ttsTimeout   = 60 * time.Second
	judgeTimeout = 20 * time.Second
)

// BattleService orchestrates the show: it drives the domain state
// machine, calls the agents and TTS through ports, and broadcasts
// every change. All mutations are serialized by a mutex — a single
// battle, a single process, a single source of truth.
type BattleService struct {
	mu sync.Mutex

	repo    domain.BattleRepository
	cache   domain.VerseCache
	writer  VerseWriter
	judge   Judge
	perform Performer
	cast    Broadcaster
	log     *slog.Logger
}

func NewBattleService(
	repo domain.BattleRepository,
	cache domain.VerseCache,
	writer VerseWriter,
	judge Judge,
	perform Performer,
	cast Broadcaster,
	log *slog.Logger,
) *BattleService {
	return &BattleService{
		repo:    repo,
		cache:   cache,
		writer:  writer,
		judge:   judge,
		perform: perform,
		cast:    cast,
		log:     log,
	}
}

// StartBattle creates a fresh battle, replacing any existing one.
func (s *BattleService) StartBattle(ctx context.Context, rounds int) (domain.BattleState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := domain.NewBattle(domain.NextID(), rounds)
	if err := s.repo.Save(ctx, b); err != nil {
		return domain.BattleState{}, err
	}
	return s.broadcastState(b), nil
}

// Reset clears the current battle (rehearsals).
func (s *BattleService) Reset(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.repo.Clear(ctx); err != nil {
		return err
	}
	s.cast.ToAudience(Event{Type: EventState, Payload: domain.BattleState{Phase: domain.PhaseIdle}})
	s.cast.ToStage(Event{Type: EventState, Payload: domain.BattleState{Phase: domain.PhaseIdle}})
	return nil
}

// State returns the current battle state.
func (s *BattleService) State(ctx context.Context) (domain.BattleState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := s.repo.Current(ctx)
	if err != nil {
		return domain.BattleState{}, err
	}
	return b.Snapshot(), nil
}

// SubmitTopic records an audience topic word.
func (s *BattleService) SubmitTopic(ctx context.Context, clientID, word string) error {
	return s.mutate(ctx, func(b *domain.Battle) error {
		return b.SubmitTopic(clientID, word)
	})
}

// Vote records an audience vote.
func (s *BattleService) Vote(ctx context.Context, clientID string, battler domain.Battler) error {
	return s.mutate(ctx, func(b *domain.Battle) error {
		return b.Vote(clientID, battler)
	})
}

// AddCrowdWord stores a live audience word (read by the crowd-scanner tool).
func (s *BattleService) AddCrowdWord(ctx context.Context, word string) error {
	return s.mutate(ctx, func(b *domain.Battle) error {
		return b.AddCrowdWord(word)
	})
}

// Advance moves the show to its next phase. Called by the presenter.
func (s *BattleService) Advance(ctx context.Context) (domain.BattleState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, err := s.repo.Current(ctx)
	if err != nil {
		return domain.BattleState{}, err
	}

	switch b.Phase() {
	case domain.PhaseIdle, domain.PhaseRoundResult:
		err = b.OpenTopics()

	case domain.PhaseTopicsOpen:
		var topic string
		if topic, err = b.StartWriting(); err == nil {
			// Verse writing runs in the background; the phase is already
			// "writing", so the stage shows both agents thinking.
			go s.writeVerses(topic, b.Snapshot().Rounds)
		}

	case domain.PhaseWriting:
		if err = b.StartPerformances(); err == nil {
			go s.performVerse(domain.BattlerGopher, b.CurrentRound().Verse(domain.BattlerGopher))
		}

	case domain.PhasePerformingA:
		if err = b.NextPerformance(); err == nil {
			go s.performVerse(domain.BattlerNullPtr, b.CurrentRound().Verse(domain.BattlerNullPtr))
		}

	case domain.PhasePerformingB:
		err = b.OpenVoting()

	case domain.PhaseVoting:
		commentary := s.judgeCommentary(b.Snapshot())
		_, err = b.CloseRound(commentary)

	default:
		err = domain.ErrInvalidPhase
	}

	if err != nil {
		return domain.BattleState{}, err
	}
	if err := s.repo.Save(ctx, b); err != nil {
		return domain.BattleState{}, err
	}
	return s.broadcastState(b), nil
}

// writeVerses asks both battlers for a verse in parallel, falling back
// to the embedded cache per battler on error or timeout.
func (s *BattleService) writeVerses(topic string, history []domain.RoundState) {
	ctx, cancel := context.WithTimeout(context.Background(), verseTimeout)
	defer cancel()

	var wg sync.WaitGroup
	for _, battler := range []domain.Battler{domain.BattlerGopher, domain.BattlerNullPtr} {
		wg.Add(1)
		go func(battler domain.Battler) {
			defer wg.Done()
			s.castStatus(battler, "writing")

			verse, err := s.writer.WriteVerse(ctx, battler, topic, history)
			status := "ready"
			if err != nil || verse == "" {
				s.log.Error("verse writing failed, using emergency verse",
					"battler", battler, "error", err)
				verse, err = s.cache.Emergency(battler, topic)
				status = "fallback"
				if err != nil {
					verse = fmt.Sprintf("(%s lost the beat — skip this round)", battler)
				}
			}

			s.mu.Lock()
			defer s.mu.Unlock()
			b, err := s.repo.Current(context.Background())
			if err != nil {
				return // battle was reset mid-write
			}
			if err := b.SetVerse(battler, verse); err != nil {
				s.log.Error("could not store verse", "battler", battler, "error", err)
				return
			}
			_ = s.repo.Save(context.Background(), b)
			s.castStatus(battler, status)
			s.broadcastState(b)
		}(battler)
	}
	wg.Wait()
}

// performVerse synthesizes audio for a verse and sends it to the stage.
// On failure the stage receives the verse text only — the presenter
// performs it live (the show must go on).
func (s *BattleService) performVerse(battler domain.Battler, verse string) {
	ctx, cancel := context.WithTimeout(context.Background(), ttsTimeout)
	defer cancel()

	payload := PerformancePayload{Battler: string(battler), Verse: verse}
	audio, err := s.perform.Synthesize(ctx, battler, verse)
	if err != nil {
		s.log.Error("tts failed", "battler", battler, "error", err)
		s.cast.ToStage(Event{Type: EventTTSUnavailable, Payload: payload})
		return
	}
	payload.AudioB64 = base64.StdEncoding.EncodeToString(audio)
	s.cast.ToStage(Event{Type: EventPerformance, Payload: payload})
}

func (s *BattleService) judgeCommentary(state domain.BattleState) string {
	ctx, cancel := context.WithTimeout(context.Background(), judgeTimeout)
	defer cancel()
	commentary, err := s.judge.Commentary(ctx, state)
	if err != nil || commentary == "" {
		s.log.Error("judge failed, using canned commentary", "error", err)
		return "The crowd has spoken — and the judge is speechless. On to the next!"
	}
	return commentary
}

// mutate applies fn to the current battle under lock, saves and broadcasts.
func (s *BattleService) mutate(ctx context.Context, fn func(*domain.Battle) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := s.repo.Current(ctx)
	if err != nil {
		return err
	}
	if err := fn(b); err != nil {
		return err
	}
	if err := s.repo.Save(ctx, b); err != nil {
		return err
	}
	s.broadcastState(b)
	return nil
}

// broadcastState pushes the full state to everyone. Callers must hold s.mu.
func (s *BattleService) broadcastState(b *domain.Battle) domain.BattleState {
	state := b.Snapshot()
	event := Event{Type: EventState, Payload: state}
	s.cast.ToAudience(event)
	s.cast.ToStage(event)
	return state
}

func (s *BattleService) castStatus(battler domain.Battler, status string) {
	event := Event{Type: EventAgentStatus, Payload: AgentStatusPayload{
		Battler: string(battler),
		Status:  status,
	}}
	s.cast.ToAudience(event)
	s.cast.ToStage(event)
}
