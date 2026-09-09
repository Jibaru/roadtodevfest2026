package persistence_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jibaru/agentarena/internal/review/domain"
	filerepo "github.com/jibaru/agentarena/internal/review/infra/persistence/file"
	"github.com/jibaru/agentarena/internal/review/infra/persistence/memory"
)

// Both repositories implement the same domain interface; the same
// suite runs against each (the "swap storage in one line" claim, tested).
func TestSessionRepositories(t *testing.T) {
	fileRepo, err := filerepo.NewSessionRepository(t.TempDir())
	require.NoError(t, err)

	repos := map[string]domain.SessionRepository{
		"memory": memory.NewSessionRepository(),
		"file":   fileRepo,
	}

	for name, repo := range repos {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			_, err := repo.Current(ctx)
			assert.ErrorIs(t, err, domain.ErrSessionNotFound)

			s := domain.NewSession("session-1")
			require.NoError(t, s.OpenRepos())
			require.NoError(t, s.SubmitRepo("c1", "golang/go"))
			require.NoError(t, repo.Save(ctx, s))

			got, err := repo.Current(ctx)
			require.NoError(t, err)
			assert.Equal(t, "session-1", got.ID())
			assert.Equal(t, domain.PhaseReposOpen, got.Phase())
			assert.Equal(t, map[string]int{"golang/go": 1}, got.CurrentRound().RepoCounts())

			require.NoError(t, repo.Clear(ctx))
			_, err = repo.Current(ctx)
			assert.ErrorIs(t, err, domain.ErrSessionNotFound)
			require.NoError(t, repo.Clear(ctx), "clearing twice is fine")
		})
	}
}
