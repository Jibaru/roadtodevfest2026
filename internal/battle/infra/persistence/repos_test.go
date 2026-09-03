package persistence_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jibaru/agentarena/internal/battle/domain"
	"github.com/jibaru/agentarena/internal/battle/infra/persistence/embedded"
	filerepo "github.com/jibaru/agentarena/internal/battle/infra/persistence/file"
	"github.com/jibaru/agentarena/internal/battle/infra/persistence/memory"
)

// Both repositories implement the same domain interface; the same
// suite runs against each (the "swap storage in one line" claim, tested).
func TestBattleRepositories(t *testing.T) {
	fileRepo, err := filerepo.NewBattleRepository(t.TempDir())
	require.NoError(t, err)

	repos := map[string]domain.BattleRepository{
		"memory": memory.NewBattleRepository(),
		"file":   fileRepo,
	}

	for name, repo := range repos {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			_, err := repo.Current(ctx)
			assert.ErrorIs(t, err, domain.ErrBattleNotFound)

			b := domain.NewBattle("battle-1", 3)
			require.NoError(t, b.OpenTopics())
			require.NoError(t, b.SubmitTopic("c1", "cats"))
			require.NoError(t, repo.Save(ctx, b))

			got, err := repo.Current(ctx)
			require.NoError(t, err)
			assert.Equal(t, "battle-1", got.ID())
			assert.Equal(t, domain.PhaseTopicsOpen, got.Phase())
			assert.Equal(t, map[string]int{"cats": 1}, got.CurrentRound().TopicCounts())

			require.NoError(t, repo.Clear(ctx))
			_, err = repo.Current(ctx)
			assert.ErrorIs(t, err, domain.ErrBattleNotFound)
			require.NoError(t, repo.Clear(ctx), "clearing twice is fine")
		})
	}
}

func TestEmbeddedVerseCache(t *testing.T) {
	cache, err := embedded.NewVerseCache()
	require.NoError(t, err)

	for _, battler := range []domain.Battler{domain.BattlerBlue, domain.BattlerRed} {
		v1, err := cache.Emergency(battler, "mondays")
		require.NoError(t, err)
		assert.Contains(t, v1, "mondays", "topic placeholder is filled")
		v2, err := cache.Emergency(battler, "mondays")
		require.NoError(t, err)
		assert.Equal(t, v1, v2, "same topic returns same verse (deterministic)")
	}

	_, err = cache.Emergency(domain.Battler("nobody"), "x")
	assert.ErrorIs(t, err, domain.ErrVerseNotCached)
}
