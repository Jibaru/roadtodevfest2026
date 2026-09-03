package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jibaru/rapbattle/internal/battle/domain"
)

func TestBattleFullHappyPath(t *testing.T) {
	b := domain.NewBattle(domain.NextID(), 1)
	assert.Equal(t, domain.PhaseIdle, b.Phase())

	require.NoError(t, b.OpenTopics())
	require.NoError(t, b.SubmitTopic("client-1", "  Mondays "))
	require.NoError(t, b.SubmitTopic("client-2", "mondays"))
	require.NoError(t, b.SubmitTopic("client-3", "php"))

	topic, err := b.StartWriting()
	require.NoError(t, err)
	assert.Equal(t, "mondays", topic, "most-submitted normalized word wins")

	require.NoError(t, b.SetVerse(domain.BattlerGopher, "gopher bars"))
	require.NoError(t, b.SetVerse(domain.BattlerNullPtr, "null bars"))
	require.NoError(t, b.StartPerformances())
	assert.Equal(t, domain.PhasePerformingA, b.Phase())
	require.NoError(t, b.NextPerformance())
	require.NoError(t, b.OpenVoting())

	require.NoError(t, b.Vote("client-1", domain.BattlerGopher))
	require.NoError(t, b.Vote("client-2", domain.BattlerGopher))
	require.NoError(t, b.Vote("client-3", domain.BattlerNullPtr))

	winner, err := b.CloseRound("gopher cooked")
	require.NoError(t, err)
	assert.Equal(t, domain.BattlerGopher, winner)
	assert.Equal(t, domain.PhaseChampion, b.Phase(), "single-round battle ends after round 1")
	assert.Equal(t, domain.BattlerGopher, b.Champion())
}

func TestBattleMultiRoundProgression(t *testing.T) {
	b := domain.NewBattle("b1", 2)
	playRound := func(voter domain.Battler) {
		require.NoError(t, b.OpenTopics())
		require.NoError(t, b.SubmitTopic("c1", "cats"))
		_, err := b.StartWriting()
		require.NoError(t, err)
		require.NoError(t, b.SetVerse(domain.BattlerGopher, "v1"))
		require.NoError(t, b.SetVerse(domain.BattlerNullPtr, "v2"))
		require.NoError(t, b.StartPerformances())
		require.NoError(t, b.NextPerformance())
		require.NoError(t, b.OpenVoting())
		require.NoError(t, b.Vote("c1", voter))
		_, err = b.CloseRound("ok")
		require.NoError(t, err)
	}

	playRound(domain.BattlerNullPtr)
	assert.Equal(t, domain.PhaseRoundResult, b.Phase())
	playRound(domain.BattlerNullPtr)
	assert.Equal(t, domain.PhaseChampion, b.Phase())
	assert.Equal(t, domain.BattlerNullPtr, b.Champion())
	assert.Equal(t, 2, b.Scores()[domain.BattlerNullPtr])

	// No more rounds after champion.
	assert.ErrorIs(t, b.OpenTopics(), domain.ErrInvalidPhase)
}

func TestBattleDeduplication(t *testing.T) {
	b := domain.NewBattle("b1", 3)
	require.NoError(t, b.OpenTopics())
	require.NoError(t, b.SubmitTopic("c1", "cats"))
	assert.ErrorIs(t, b.SubmitTopic("c1", "dogs"), domain.ErrAlreadySubmitted)

	_, err := b.StartWriting()
	require.NoError(t, err)
	require.NoError(t, b.SetVerse(domain.BattlerGopher, "v"))
	require.NoError(t, b.SetVerse(domain.BattlerNullPtr, "v"))
	require.NoError(t, b.StartPerformances())
	require.NoError(t, b.NextPerformance())
	require.NoError(t, b.OpenVoting())
	require.NoError(t, b.Vote("c1", domain.BattlerGopher))
	assert.ErrorIs(t, b.Vote("c1", domain.BattlerNullPtr), domain.ErrAlreadyVoted)
}

func TestBattlePhaseGuards(t *testing.T) {
	b := domain.NewBattle("b1", 3)
	assert.ErrorIs(t, b.SubmitTopic("c1", "x"), domain.ErrInvalidPhase)
	_, err := b.StartWriting()
	assert.ErrorIs(t, err, domain.ErrInvalidPhase)
	assert.ErrorIs(t, b.Vote("c1", domain.BattlerGopher), domain.ErrInvalidPhase)

	require.NoError(t, b.OpenTopics())
	// Writing cannot start with zero topics.
	_, err = b.StartWriting()
	assert.ErrorIs(t, err, domain.ErrNoTopics)
	// Performances cannot start without both verses.
	require.NoError(t, b.SubmitTopic("c1", "x"))
	_, err = b.StartWriting()
	require.NoError(t, err)
	require.NoError(t, b.SetVerse(domain.BattlerGopher, "only one"))
	assert.ErrorIs(t, b.StartPerformances(), domain.ErrInvalidPhase)
}

func TestSnapshotRoundTrip(t *testing.T) {
	b := domain.NewBattle("b1", 3)
	require.NoError(t, b.OpenTopics())
	require.NoError(t, b.SubmitTopic("c1", "cats"))
	_, err := b.StartWriting()
	require.NoError(t, err)
	require.NoError(t, b.SetVerse(domain.BattlerGopher, "bars"))
	require.NoError(t, b.AddCrowdWord("fire"))

	restored := domain.RestoreBattle(b.Snapshot())
	assert.Equal(t, b.Phase(), restored.Phase())
	assert.Equal(t, "cats", restored.CurrentRound().Topic())
	assert.Equal(t, "bars", restored.CurrentRound().Verse(domain.BattlerGopher))
	assert.Equal(t, []string{"fire"}, restored.RecentCrowdWords(5))

	// The restored aggregate keeps enforcing invariants (e.g. topic dedup state).
	require.NoError(t, restored.SetVerse(domain.BattlerNullPtr, "cold bars"))
	require.NoError(t, restored.StartPerformances())
}

func TestCrowdWordsRing(t *testing.T) {
	b := domain.NewBattle("b1", 3)
	for i := 0; i < 60; i++ {
		require.NoError(t, b.AddCrowdWord("w"+string(rune('a'+i%26))))
	}
	words := b.RecentCrowdWords(100)
	assert.Len(t, words, 50, "ring keeps only the most recent 50")
	assert.ErrorIs(t, b.AddCrowdWord("   "), domain.ErrEmptyWord)
}
