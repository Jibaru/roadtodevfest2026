package service_test

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jibaru/rapbattle/internal/battle/domain"
	"github.com/jibaru/rapbattle/internal/battle/infra/persistence/embedded"
	"github.com/jibaru/rapbattle/internal/battle/infra/persistence/memory"
	"github.com/jibaru/rapbattle/internal/battle/service"
)

type fakeWriter struct{ err error }

func (f *fakeWriter) WriteVerse(_ context.Context, battler domain.Battler, topic string, _ []domain.RoundState) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return string(battler) + " raps about " + topic, nil
}

type fakeJudge struct{ err error }

func (f *fakeJudge) Commentary(context.Context, domain.BattleState) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return "what a round!", nil
}

type fakePerformer struct{ err error }

func (f *fakePerformer) Synthesize(_ context.Context, _ domain.Battler, _ string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []byte("RIFF-fake-wav"), nil
}

type fakeCast struct {
	mu     sync.Mutex
	stage  []service.Event
	crowd  []service.Event
}

func (f *fakeCast) ToAudience(e service.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.crowd = append(f.crowd, e)
}

func (f *fakeCast) ToStage(e service.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stage = append(f.stage, e)
}

func (f *fakeCast) stageEvents(eventType string) []service.Event {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []service.Event
	for _, e := range f.stage {
		if e.Type == eventType {
			out = append(out, e)
		}
	}
	return out
}

func newService(t *testing.T, writer service.VerseWriter, judge service.Judge, performer service.Performer) (*service.BattleService, *fakeCast) {
	t.Helper()
	cache, err := embedded.NewVerseCache()
	require.NoError(t, err)
	cast := &fakeCast{}
	svc := service.NewBattleService(
		memory.NewBattleRepository(), cache, writer, judge, performer, cast,
		slog.New(slog.DiscardHandler),
	)
	return svc, cast
}

func waitForPhase(t *testing.T, svc *service.BattleService, phase domain.Phase) domain.BattleState {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		state, err := svc.State(context.Background())
		require.NoError(t, err)
		if state.Phase == phase {
			return state
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for phase %s", phase)
	return domain.BattleState{}
}

func waitForVerses(t *testing.T, svc *service.BattleService) domain.BattleState {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		state, err := svc.State(context.Background())
		require.NoError(t, err)
		round := state.Rounds[len(state.Rounds)-1]
		if round.Verses[domain.BattlerGopher] != "" && round.Verses[domain.BattlerNullPtr] != "" {
			return state
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for verses")
	return domain.BattleState{}
}

func TestFullShowFlow(t *testing.T) {
	svc, cast := newService(t, &fakeWriter{}, &fakeJudge{}, &fakePerformer{})
	ctx := context.Background()

	state, err := svc.StartBattle(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, domain.PhaseIdle, state.Phase)

	_, err = svc.Advance(ctx) // idle -> topics_open
	require.NoError(t, err)
	require.NoError(t, svc.SubmitTopic(ctx, "c1", "mondays"))
	require.NoError(t, svc.SubmitTopic(ctx, "c2", "mondays"))

	_, err = svc.Advance(ctx) // topics_open -> writing (agents kick off)
	require.NoError(t, err)
	state = waitForVerses(t, svc)
	assert.Equal(t, "gopher raps about mondays", state.Rounds[0].Verses[domain.BattlerGopher])

	_, err = svc.Advance(ctx) // writing -> performing_a
	require.NoError(t, err)
	_, err = svc.Advance(ctx) // performing_a -> performing_b
	require.NoError(t, err)
	_, err = svc.Advance(ctx) // performing_b -> voting
	require.NoError(t, err)

	require.NoError(t, svc.Vote(ctx, "c1", domain.BattlerNullPtr))
	state, err = svc.Advance(ctx) // voting -> champion (single round)
	require.NoError(t, err)
	assert.Equal(t, domain.PhaseChampion, state.Phase)
	assert.Equal(t, domain.BattlerNullPtr, state.Champion)
	assert.Equal(t, "what a round!", state.Rounds[0].JudgeCommentary)

	// Both performances reached the stage with audio.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(cast.stageEvents(service.EventPerformance)) == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	assert.Len(t, cast.stageEvents(service.EventPerformance), 2)
}

func TestWriterFailureFallsBackToEmergencyVerses(t *testing.T) {
	svc, _ := newService(t, &fakeWriter{err: errors.New("gemini down")}, &fakeJudge{}, &fakePerformer{})
	ctx := context.Background()

	_, err := svc.StartBattle(ctx, 1)
	require.NoError(t, err)
	_, err = svc.Advance(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.SubmitTopic(ctx, "c1", "kubernetes"))
	_, err = svc.Advance(ctx)
	require.NoError(t, err)

	state := waitForVerses(t, svc)
	assert.Contains(t, state.Rounds[0].Verses[domain.BattlerGopher], "kubernetes",
		"emergency verse has the topic substituted")
}

func TestTTSFailureNotifiesStage(t *testing.T) {
	svc, cast := newService(t, &fakeWriter{}, &fakeJudge{}, &fakePerformer{err: errors.New("tts down")})
	ctx := context.Background()

	_, err := svc.StartBattle(ctx, 1)
	require.NoError(t, err)
	_, err = svc.Advance(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.SubmitTopic(ctx, "c1", "bugs"))
	_, err = svc.Advance(ctx)
	require.NoError(t, err)
	waitForVerses(t, svc)
	_, err = svc.Advance(ctx) // -> performing_a triggers TTS
	require.NoError(t, err)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(cast.stageEvents(service.EventTTSUnavailable)) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	events := cast.stageEvents(service.EventTTSUnavailable)
	require.NotEmpty(t, events, "stage is told to fall back to a live reading")
	payload := events[0].Payload.(service.PerformancePayload)
	assert.NotEmpty(t, payload.Verse, "verse text still delivered for the presenter")
}

func TestJudgeFailureUsesCannedLine(t *testing.T) {
	svc, _ := newService(t, &fakeWriter{}, &fakeJudge{err: errors.New("no comment")}, &fakePerformer{})
	ctx := context.Background()

	_, err := svc.StartBattle(ctx, 1)
	require.NoError(t, err)
	_, err = svc.Advance(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.SubmitTopic(ctx, "c1", "rust"))
	_, err = svc.Advance(ctx)
	require.NoError(t, err)
	waitForVerses(t, svc)
	for i := 0; i < 3; i++ { // -> performing_a -> performing_b -> voting
		_, err = svc.Advance(ctx)
		require.NoError(t, err)
	}
	state, err := svc.Advance(ctx) // close round
	require.NoError(t, err)
	assert.NotEmpty(t, state.Rounds[0].JudgeCommentary)
}
