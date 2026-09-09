package domain

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Round is one repository reviewed in front of the audience.
type Round struct {
	number      int
	reposBy     map[string]string // clientID -> normalized owner/repo
	repo        string            // chosen owner/repo
	findings    []Finding
	votesBy     map[string]string // clientID -> findingID
	summary     string
	nextFinding int
}

func newRound(number int) *Round {
	return &Round{
		number:  number,
		reposBy: map[string]string{},
		votesBy: map[string]string{},
	}
}

func (r *Round) Number() int     { return r.number }
func (r *Round) Repo() string    { return r.repo }
func (r *Round) Summary() string { return r.summary }

// Findings returns the findings sorted by severity (high first).
func (r *Round) Findings() []Finding {
	out := append([]Finding(nil), r.findings...)
	sort.SliceStable(out, func(i, j int) bool {
		return severityRank(out[i].Severity) < severityRank(out[j].Severity)
	})
	return out
}

// RepoCounts tallies submitted repos.
func (r *Round) RepoCounts() map[string]int {
	counts := map[string]int{}
	for _, repo := range r.reposBy {
		counts[repo]++
	}
	return counts
}

// VoteCounts tallies votes per finding ID.
func (r *Round) VoteCounts() map[string]int {
	counts := map[string]int{}
	for _, id := range r.votesBy {
		counts[id]++
	}
	return counts
}

// Session is the aggregate root: a live review show with N rounds.
type Session struct {
	id     string
	phase  Phase
	rounds []*Round
}

// NewSession creates a session in the idle phase.
func NewSession(id string) *Session {
	return &Session{id: id, phase: PhaseIdle}
}

func (s *Session) ID() string       { return s.id }
func (s *Session) Phase() Phase     { return s.phase }
func (s *Session) Rounds() []*Round { return s.rounds }

// CurrentRound returns the round in progress, or nil before the first one.
func (s *Session) CurrentRound() *Round {
	if len(s.rounds) == 0 {
		return nil
	}
	return s.rounds[len(s.rounds)-1]
}

// Scores counts finding votes per reviewer across all rounds:
// the reviewers compete for the most valuable findings.
func (s *Session) Scores() map[Reviewer]int {
	scores := map[Reviewer]int{}
	for _, rv := range AllReviewers {
		scores[rv] = 0
	}
	for _, r := range s.rounds {
		byID := map[string]Reviewer{}
		for _, f := range r.findings {
			byID[f.ID] = f.Reviewer
		}
		for _, fid := range r.votesBy {
			if rv, ok := byID[fid]; ok {
				scores[rv]++
			}
		}
	}
	return scores
}

// OpenRepos starts the next round and opens repo submissions.
func (s *Session) OpenRepos() error {
	if s.phase != PhaseIdle && s.phase != PhaseResults {
		return ErrInvalidPhase
	}
	s.rounds = append(s.rounds, newRound(len(s.rounds)+1))
	s.phase = PhaseReposOpen
	return nil
}

// SubmitRepo records one repo per client per round. Submitting a repo
// someone else already proposed counts as a vote for it.
func (s *Session) SubmitRepo(clientID, raw string) error {
	if s.phase != PhaseReposOpen {
		return ErrInvalidPhase
	}
	repo, err := NormalizeRepo(raw)
	if err != nil {
		return err
	}
	r := s.CurrentRound()
	if _, dup := r.reposBy[clientID]; dup {
		return ErrAlreadySubmitted
	}
	r.reposBy[clientID] = repo
	return nil
}

// StartReview closes submissions, picks the most-proposed repo and
// moves to the reviewing phase. Returns "owner/repo".
func (s *Session) StartReview() (string, error) {
	if s.phase != PhaseReposOpen {
		return "", ErrInvalidPhase
	}
	r := s.CurrentRound()
	counts := r.RepoCounts()
	if len(counts) == 0 {
		return "", ErrNoRepos
	}
	type rc struct {
		repo  string
		count int
	}
	ranked := make([]rc, 0, len(counts))
	for repo, c := range counts {
		ranked = append(ranked, rc{repo, c})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].count != ranked[j].count {
			return ranked[i].count > ranked[j].count
		}
		return ranked[i].repo < ranked[j].repo // deterministic tie-break
	})
	r.repo = ranked[0].repo
	s.phase = PhaseReviewing
	return r.repo, nil
}

// AddFindings stores a reviewer's findings during the reviewing phase,
// assigning each a round-scoped ID.
func (s *Session) AddFindings(reviewer Reviewer, findings []Finding) error {
	if s.phase != PhaseReviewing {
		return ErrInvalidPhase
	}
	if !reviewer.Valid() {
		return ErrInvalidReviewer
	}
	r := s.CurrentRound()
	for _, f := range findings {
		r.nextFinding++
		f.ID = fmt.Sprintf("r%d-f%d", r.number, r.nextFinding)
		f.Reviewer = reviewer
		f.Severity = normalizeSeverity(f.Severity)
		if strings.TrimSpace(f.Title) == "" {
			continue
		}
		r.findings = append(r.findings, f)
	}
	return nil
}

// FinishReview stores the lead reviewer's summary and opens voting.
func (s *Session) FinishReview(summary string) error {
	if s.phase != PhaseReviewing {
		return ErrInvalidPhase
	}
	s.CurrentRound().summary = summary
	s.phase = PhaseResults
	return nil
}

// VoteFinding records one most-valuable-finding vote per client per round.
func (s *Session) VoteFinding(clientID, findingID string) error {
	if s.phase != PhaseResults {
		return ErrInvalidPhase
	}
	r := s.CurrentRound()
	found := false
	for _, f := range r.findings {
		if f.ID == findingID {
			found = true
			break
		}
	}
	if !found {
		return ErrFindingNotFound
	}
	if _, dup := r.votesBy[clientID]; dup {
		return ErrAlreadyVoted
	}
	r.votesBy[clientID] = findingID
	return nil
}

var repoRe = regexp.MustCompile(`^([A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?)/([A-Za-z0-9._-]+?)(?:\.git)?$`)

// NormalizeRepo accepts "https://github.com/owner/repo", "github.com/owner/repo"
// or "owner/repo" and returns the canonical "owner/repo".
func NormalizeRepo(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	v = strings.TrimPrefix(v, "https://")
	v = strings.TrimPrefix(v, "http://")
	v = strings.TrimPrefix(v, "www.")
	v = strings.TrimPrefix(v, "github.com/")
	v = strings.Trim(v, "/")
	if len(v) > 120 {
		return "", ErrInvalidRepoURL
	}
	m := repoRe.FindStringSubmatch(v)
	if m == nil {
		return "", ErrInvalidRepoURL
	}
	return strings.ToLower(m[1]) + "/" + strings.ToLower(m[2]), nil
}
