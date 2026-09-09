package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jibaru/agentarena/internal/review/domain"
)

func TestNormalizeRepo(t *testing.T) {
	for raw, want := range map[string]string{
		"https://github.com/Jibaru/AgentArena": "jibaru/agentarena",
		"github.com/golang/go/":                "golang/go",
		"gin-gonic/gin.git":                    "gin-gonic/gin",
		"  http://www.github.com/a/b  ":        "a/b",
	} {
		got, err := domain.NormalizeRepo(raw)
		require.NoError(t, err, raw)
		assert.Equal(t, want, got)
	}
	for _, bad := range []string{"", "justoneword", "https://gitlab.com/a/b?x=1", "a/b/c/d", "-bad/repo"} {
		_, err := domain.NormalizeRepo(bad)
		assert.ErrorIs(t, err, domain.ErrInvalidRepoURL, bad)
	}
}

func playToReviewing(t *testing.T, s *domain.Session) string {
	t.Helper()
	require.NoError(t, s.OpenRepos())
	require.NoError(t, s.SubmitRepo("c1", "golang/go"))
	require.NoError(t, s.SubmitRepo("c2", "github.com/golang/go"))
	require.NoError(t, s.SubmitRepo("c3", "gin-gonic/gin"))
	repo, err := s.StartReview()
	require.NoError(t, err)
	return repo
}

func TestFullReviewFlow(t *testing.T) {
	s := domain.NewSession(domain.NextID())
	assert.Equal(t, domain.PhaseIdle, s.Phase())

	repo := playToReviewing(t, s)
	assert.Equal(t, "golang/go", repo, "most-proposed repo wins")

	require.NoError(t, s.AddFindings(domain.ReviewerBugs, []domain.Finding{
		{Severity: "low", Title: "Minor bug", Detail: "d"},
		{Severity: "high", Title: "Data race", Detail: "d", File: "main.go"},
	}))
	require.NoError(t, s.AddFindings(domain.ReviewerSecurity, []domain.Finding{
		{Severity: "whatever", Title: "Weird severity is normalized", Detail: "d"},
		{Title: "   ", Detail: "empty titles are dropped"},
	}))
	require.NoError(t, s.FinishReview("solid repo overall"))
	assert.Equal(t, domain.PhaseResults, s.Phase())

	findings := s.CurrentRound().Findings()
	require.Len(t, findings, 3)
	assert.Equal(t, "Data race", findings[0].Title, "high severity sorts first")
	assert.Equal(t, "medium", findings[1].Severity, "unknown severity normalized")

	// Voting: attribute to reviewers.
	raceID := findings[0].ID
	require.NoError(t, s.VoteFinding("c1", raceID))
	require.NoError(t, s.VoteFinding("c2", raceID))
	assert.ErrorIs(t, s.VoteFinding("c1", raceID), domain.ErrAlreadyVoted)
	assert.ErrorIs(t, s.VoteFinding("c3", "nope"), domain.ErrFindingNotFound)
	assert.Equal(t, 2, s.Scores()[domain.ReviewerBugs])
	assert.Equal(t, 0, s.Scores()[domain.ReviewerSecurity])

	// Next round loops back.
	require.NoError(t, s.OpenRepos())
	assert.Equal(t, domain.PhaseReposOpen, s.Phase())
	assert.Equal(t, 2, s.CurrentRound().Number())
}

func TestPhaseGuardsAndDedup(t *testing.T) {
	s := domain.NewSession("s1")
	assert.ErrorIs(t, s.SubmitRepo("c1", "a/b"), domain.ErrInvalidPhase)
	_, err := s.StartReview()
	assert.ErrorIs(t, err, domain.ErrInvalidPhase)
	assert.ErrorIs(t, s.VoteFinding("c1", "x"), domain.ErrInvalidPhase)

	require.NoError(t, s.OpenRepos())
	_, err = s.StartReview()
	assert.ErrorIs(t, err, domain.ErrNoRepos)

	require.NoError(t, s.SubmitRepo("c1", "a/b"))
	assert.ErrorIs(t, s.SubmitRepo("c1", "c/d"), domain.ErrAlreadySubmitted)
	assert.ErrorIs(t, s.SubmitRepo("c2", "not a url at all !!"), domain.ErrInvalidRepoURL)

	_, err = s.StartReview()
	require.NoError(t, err)
	assert.ErrorIs(t, s.AddFindings("nobody", nil), domain.ErrInvalidReviewer)
	require.NoError(t, s.FinishReview("ok"))
	assert.ErrorIs(t, s.FinishReview("twice"), domain.ErrInvalidPhase)
}

func TestSnapshotRoundTrip(t *testing.T) {
	s := domain.NewSession("s1")
	playToReviewing(t, s)
	require.NoError(t, s.AddFindings(domain.ReviewerSimplify, []domain.Finding{
		{Severity: "high", Title: "Too clever", Detail: "d"},
	}))

	restored := domain.RestoreSession(s.Snapshot())
	assert.Equal(t, domain.PhaseReviewing, restored.Phase())
	assert.Equal(t, "golang/go", restored.CurrentRound().Repo())
	require.Len(t, restored.CurrentRound().Findings(), 1)

	// Restored aggregate keeps enforcing invariants and continues the flow;
	// new finding IDs don't collide with restored ones.
	require.NoError(t, restored.AddFindings(domain.ReviewerBugs, []domain.Finding{{Title: "New", Detail: "d"}}))
	ids := map[string]bool{}
	for _, f := range restored.CurrentRound().Findings() {
		assert.False(t, ids[f.ID], "duplicate finding ID %s", f.ID)
		ids[f.ID] = true
	}
	require.NoError(t, restored.FinishReview("done"))
}
