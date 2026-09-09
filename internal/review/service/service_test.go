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

	"github.com/jibaru/agentarena/internal/repofetch"
	"github.com/jibaru/agentarena/internal/review/domain"
	"github.com/jibaru/agentarena/internal/review/infra/persistence/memory"
	"github.com/jibaru/agentarena/internal/review/service"
)

type fakeFetcher struct{ err error }

func (f *fakeFetcher) Fetch(_ context.Context, owner, repo string) (*repofetch.Snapshot, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &repofetch.Snapshot{Owner: owner, Repo: repo, Files: []repofetch.File{{Path: "main.go", Size: 10}}}, nil
}

type fakeCrew struct{ err error }

func (f *fakeCrew) Review(_ context.Context, reviewer domain.Reviewer, snap *repofetch.Snapshot, onStatus func(string)) ([]domain.Finding, error) {
	onStatus("reading main.go")
	if f.err != nil {
		return nil, f.err
	}
	return []domain.Finding{{Severity: "high", Title: string(reviewer) + " finding in " + snap.FullName(), Detail: "d"}}, nil
}

type fakeLead struct{ err error }

func (f *fakeLead) Summary(_ context.Context, repo string, findings []domain.Finding) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return "summary of " + repo, nil
}

type fakeCast struct {
	mu     sync.Mutex
	events []service.Event
}

func (f *fakeCast) ToAudience(e service.Event) { f.add(e) }
func (f *fakeCast) ToStage(e service.Event)    { f.add(e) }
func (f *fakeCast) add(e service.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, e)
}

func newService(fetcher service.RepoFetcher, crew service.ReviewerAgent, lead service.LeadReviewer) *service.ReviewService {
	return service.NewReviewService(
		memory.NewSessionRepository(), fetcher, crew, lead, &fakeCast{},
		slog.New(slog.DiscardHandler),
	)
}

func waitForPhase(t *testing.T, svc *service.ReviewService, phase domain.Phase) domain.SessionState {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		state, err := svc.State(context.Background())
		require.NoError(t, err)
		if state.Phase == phase {
			return state
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for phase %s", phase)
	return domain.SessionState{}
}

func startRound(t *testing.T, svc *service.ReviewService) {
	t.Helper()
	ctx := context.Background()
	_, err := svc.StartSession(ctx)
	require.NoError(t, err)
	_, err = svc.Advance(ctx) // idle -> repos_open
	require.NoError(t, err)
	require.NoError(t, svc.SubmitRepo(ctx, "c1", "golang/go"))
	_, err = svc.Advance(ctx) // repos_open -> reviewing (async crew)
	require.NoError(t, err)
}

func TestFullReviewShow(t *testing.T) {
	svc := newService(&fakeFetcher{}, &fakeCrew{}, &fakeLead{})
	ctx := context.Background()

	startRound(t, svc)
	state := waitForPhase(t, svc, domain.PhaseResults)

	round := state.Rounds[0]
	assert.Equal(t, "golang/go", round.Repo)
	assert.Len(t, round.Findings, 3, "one finding per reviewer")
	assert.Equal(t, "summary of golang/go", round.Summary)

	// Vote and check reviewer scoreboard.
	var bugsFinding string
	for _, f := range round.Findings {
		if f.Reviewer == domain.ReviewerBugs {
			bugsFinding = f.ID
		}
	}
	require.NoError(t, svc.VoteFinding(ctx, "c1", bugsFinding))
	state, err := svc.State(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, state.Scores[domain.ReviewerBugs])

	// Next round loops.
	state, err = svc.Advance(ctx)
	require.NoError(t, err)
	assert.Equal(t, domain.PhaseReposOpen, state.Phase)
	assert.Equal(t, 2, state.Rounds[1].Number)
}

func TestFetchFailureDegradesGracefully(t *testing.T) {
	svc := newService(&fakeFetcher{err: errors.New("repo is private")}, &fakeCrew{}, &fakeLead{})
	startRound(t, svc)
	state := waitForPhase(t, svc, domain.PhaseResults)
	assert.Contains(t, state.Rounds[0].Summary, "Could not fetch golang/go")
	assert.Empty(t, state.Rounds[0].Findings)
	// The show continues: presenter can open the next round.
	_, err := svc.Advance(context.Background())
	require.NoError(t, err)
}

func TestReviewerFailureBecomesVisibleFinding(t *testing.T) {
	svc := newService(&fakeFetcher{}, &fakeCrew{err: errors.New("gemini down")}, &fakeLead{})
	startRound(t, svc)
	state := waitForPhase(t, svc, domain.PhaseResults)
	require.Len(t, state.Rounds[0].Findings, 3)
	for _, f := range state.Rounds[0].Findings {
		assert.Equal(t, "Reviewer went offline", f.Title)
	}
}

func TestLeadFailureUsesCannedSummary(t *testing.T) {
	svc := newService(&fakeFetcher{}, &fakeCrew{}, &fakeLead{err: errors.New("no comment")})
	startRound(t, svc)
	state := waitForPhase(t, svc, domain.PhaseResults)
	assert.Contains(t, state.Rounds[0].Summary, "3 things worth your attention")
}
