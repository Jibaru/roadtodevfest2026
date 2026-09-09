package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jibaru/agentarena/internal/review/domain"
)

const (
	fetchTimeout   = 45 * time.Second
	reviewTimeout  = 3 * time.Minute
	summaryTimeout = 30 * time.Second
)

// ReviewService orchestrates the show: it drives the domain state
// machine, fetches repos, runs the reviewer agents through ports and
// broadcasts every change. All mutations are serialized by a mutex —
// one session, one process, one source of truth.
type ReviewService struct {
	mu sync.Mutex

	repo    domain.SessionRepository
	fetcher RepoFetcher
	crew    ReviewerAgent
	lead    LeadReviewer
	cast    Broadcaster
	log     *slog.Logger
}

func NewReviewService(
	repo domain.SessionRepository,
	fetcher RepoFetcher,
	crew ReviewerAgent,
	lead LeadReviewer,
	cast Broadcaster,
	log *slog.Logger,
) *ReviewService {
	return &ReviewService{repo: repo, fetcher: fetcher, crew: crew, lead: lead, cast: cast, log: log}
}

// StartSession creates a fresh session, replacing any existing one.
func (s *ReviewService) StartSession(ctx context.Context) (domain.SessionState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := domain.NewSession(domain.NextID())
	if err := s.repo.Save(ctx, sess); err != nil {
		return domain.SessionState{}, err
	}
	return s.broadcastState(sess), nil
}

// Reset clears the current session (rehearsals).
func (s *ReviewService) Reset(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.repo.Clear(ctx); err != nil {
		return err
	}
	empty := Event{Type: EventState, Payload: domain.SessionState{Phase: domain.PhaseIdle}}
	s.cast.ToAudience(empty)
	s.cast.ToStage(empty)
	return nil
}

// State returns the current session state.
func (s *ReviewService) State(ctx context.Context) (domain.SessionState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, err := s.repo.Current(ctx)
	if err != nil {
		return domain.SessionState{}, err
	}
	return sess.Snapshot(), nil
}

// SubmitRepo records an audience repo proposal.
func (s *ReviewService) SubmitRepo(ctx context.Context, clientID, url string) error {
	return s.mutate(ctx, func(sess *domain.Session) error {
		return sess.SubmitRepo(clientID, url)
	})
}

// VoteFinding records an audience most-valuable-finding vote.
func (s *ReviewService) VoteFinding(ctx context.Context, clientID, findingID string) error {
	return s.mutate(ctx, func(sess *domain.Session) error {
		return sess.VoteFinding(clientID, findingID)
	})
}

// Advance moves the show to its next phase. Called by the presenter.
// reviewing → results happens automatically when the agents finish.
func (s *ReviewService) Advance(ctx context.Context) (domain.SessionState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, err := s.repo.Current(ctx)
	if err != nil {
		return domain.SessionState{}, err
	}

	switch sess.Phase() {
	case domain.PhaseIdle, domain.PhaseResults:
		err = sess.OpenRepos()

	case domain.PhaseReposOpen:
		var repoFull string
		if repoFull, err = sess.StartReview(); err == nil {
			// The whole review runs in the background; the phase is
			// already "reviewing", so everyone watches the agents work.
			go s.runReview(repoFull)
		}

	default:
		err = domain.ErrInvalidPhase
	}

	if err != nil {
		return domain.SessionState{}, err
	}
	if err := s.repo.Save(ctx, sess); err != nil {
		return domain.SessionState{}, err
	}
	return s.broadcastState(sess), nil
}

// runReview fetches the repo, runs the three reviewers in parallel and
// closes the round with the lead reviewer's summary. Every failure
// degrades into a visible finding — the show never stops.
func (s *ReviewService) runReview(repoFull string) {
	owner, name, _ := strings.Cut(repoFull, "/")

	fctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	snap, err := s.fetcher.Fetch(fctx, owner, name)
	cancel()
	if err != nil {
		s.log.Error("repo fetch failed", "repo", repoFull, "error", err)
		s.finish(fmt.Sprintf("Could not fetch %s: %v. Pick another repo and advance again!", repoFull, err))
		return
	}
	s.log.Info("repo fetched", "repo", repoFull, "files", len(snap.Files))

	ctx, cancelReview := context.WithTimeout(context.Background(), reviewTimeout)
	defer cancelReview()

	var wg sync.WaitGroup
	for _, reviewer := range domain.AllReviewers {
		wg.Add(1)
		go func(reviewer domain.Reviewer) {
			defer wg.Done()
			s.castStatus(reviewer, "warming up", false)

			findings, err := s.crew.Review(ctx, reviewer, snap, func(status string) {
				s.castStatus(reviewer, status, false)
			})
			if err != nil || len(findings) == 0 {
				s.log.Error("reviewer failed", "reviewer", reviewer, "error", err)
				findings = []domain.Finding{{
					Severity: domain.SeverityLow,
					Title:    "Reviewer went offline",
					Detail:   fmt.Sprintf("The %s reviewer could not finish this round.", reviewer),
				}}
			}

			s.mu.Lock()
			defer s.mu.Unlock()
			sess, err := s.repo.Current(context.Background())
			if err != nil {
				return // session was reset mid-review
			}
			if err := sess.AddFindings(reviewer, findings); err != nil {
				s.log.Error("could not store findings", "reviewer", reviewer, "error", err)
				return
			}
			_ = s.repo.Save(context.Background(), sess)
			s.castStatus(reviewer, fmt.Sprintf("done: %d findings", len(findings)), true)
			s.broadcastState(sess)
		}(reviewer)
	}
	wg.Wait()

	s.finish(s.leadSummary(repoFull))
}

// finish closes the reviewing phase with a summary and broadcasts.
func (s *ReviewService) finish(summary string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, err := s.repo.Current(context.Background())
	if err != nil {
		return
	}
	if err := sess.FinishReview(summary); err != nil {
		s.log.Error("finish review", "error", err)
		return
	}
	_ = s.repo.Save(context.Background(), sess)
	s.broadcastState(sess)
}

func (s *ReviewService) leadSummary(repoFull string) string {
	state, err := s.State(context.Background())
	if err != nil || len(state.Rounds) == 0 {
		return "That's a wrap on this repo!"
	}
	round := state.Rounds[len(state.Rounds)-1]

	ctx, cancel := context.WithTimeout(context.Background(), summaryTimeout)
	defer cancel()
	summary, err := s.lead.Summary(ctx, repoFull, round.Findings)
	if err != nil || summary == "" {
		s.log.Error("lead reviewer failed, using canned summary", "error", err)
		return fmt.Sprintf("The crew found %d things worth your attention — vote for the most valuable one!", len(round.Findings))
	}
	return summary
}

// mutate applies fn to the current session under lock, saves and broadcasts.
func (s *ReviewService) mutate(ctx context.Context, fn func(*domain.Session) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, err := s.repo.Current(ctx)
	if err != nil {
		return err
	}
	if err := fn(sess); err != nil {
		return err
	}
	if err := s.repo.Save(ctx, sess); err != nil {
		return err
	}
	s.broadcastState(sess)
	return nil
}

// broadcastState pushes the full state to everyone. Callers must hold s.mu.
func (s *ReviewService) broadcastState(sess *domain.Session) domain.SessionState {
	state := sess.Snapshot()
	event := Event{Type: EventState, Payload: state}
	s.cast.ToAudience(event)
	s.cast.ToStage(event)
	return state
}

func (s *ReviewService) castStatus(reviewer domain.Reviewer, status string, done bool) {
	event := Event{Type: EventAgentStatus, Payload: AgentStatusPayload{
		Reviewer: string(reviewer),
		Status:   status,
		Done:     done,
	}}
	s.cast.ToAudience(event)
	s.cast.ToStage(event)
}
