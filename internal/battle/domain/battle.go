package domain

import (
	"sort"
	"strings"
)

// Round holds everything that happens in one battle round.
type Round struct {
	number      int
	topic       string
	topicsBy    map[string]string // clientID -> normalized word
	verses      map[Battler]string
	votesBy     map[string]Battler // clientID -> battler voted for
	judgeSay    string
	roundWinner Battler // empty on tie
}

func newRound(number int) *Round {
	return &Round{
		number:   number,
		topicsBy: map[string]string{},
		verses:   map[Battler]string{},
		votesBy:  map[string]Battler{},
	}
}

func (r *Round) Number() int              { return r.number }
func (r *Round) Topic() string            { return r.topic }
func (r *Round) Verse(b Battler) string   { return r.verses[b] }
func (r *Round) JudgeCommentary() string  { return r.judgeSay }
func (r *Round) Winner() Battler          { return r.roundWinner }

// VoteCounts tallies votes per battler.
func (r *Round) VoteCounts() map[Battler]int {
	counts := map[Battler]int{BattlerBlue: 0, BattlerRed: 0}
	for _, b := range r.votesBy {
		counts[b]++
	}
	return counts
}

// TopicCounts tallies submitted words.
func (r *Round) TopicCounts() map[string]int {
	counts := map[string]int{}
	for _, w := range r.topicsBy {
		counts[w]++
	}
	return counts
}

// Battle is the aggregate root: a best-of-N rap battle.
type Battle struct {
	id          string
	phase       Phase
	totalRounds int
	rounds      []*Round
	crowdWords  []string // live words from the audience, read by the crowd-scanner tool
}

// NewBattle creates a battle in the idle phase.
func NewBattle(id string, totalRounds int) *Battle {
	if totalRounds <= 0 {
		totalRounds = 3
	}
	return &Battle{id: id, phase: PhaseIdle, totalRounds: totalRounds}
}

func (b *Battle) ID() string       { return b.id }
func (b *Battle) Phase() Phase     { return b.phase }
func (b *Battle) TotalRounds() int { return b.totalRounds }
func (b *Battle) Rounds() []*Round { return b.rounds }

// CurrentRound returns the round in progress, or nil before the first one.
func (b *Battle) CurrentRound() *Round {
	if len(b.rounds) == 0 {
		return nil
	}
	return b.rounds[len(b.rounds)-1]
}

// Scores counts round wins per battler.
func (b *Battle) Scores() map[Battler]int {
	scores := map[Battler]int{BattlerBlue: 0, BattlerRed: 0}
	for _, r := range b.rounds {
		if r.roundWinner.Valid() {
			scores[r.roundWinner]++
		}
	}
	return scores
}

// Champion returns the overall winner once the battle is over (empty on tie).
func (b *Battle) Champion() Battler {
	if b.phase != PhaseChampion {
		return ""
	}
	scores := b.Scores()
	switch {
	case scores[BattlerBlue] > scores[BattlerRed]:
		return BattlerBlue
	case scores[BattlerRed] > scores[BattlerBlue]:
		return BattlerRed
	default:
		return ""
	}
}

// OpenTopics starts the next round and opens topic submission.
func (b *Battle) OpenTopics() error {
	if b.phase != PhaseIdle && b.phase != PhaseRoundResult {
		return ErrInvalidPhase
	}
	if len(b.rounds) >= b.totalRounds {
		return ErrInvalidPhase
	}
	b.rounds = append(b.rounds, newRound(len(b.rounds)+1))
	b.phase = PhaseTopicsOpen
	return nil
}

// SubmitTopic records one topic word per client per round.
func (b *Battle) SubmitTopic(clientID, word string) error {
	if b.phase != PhaseTopicsOpen {
		return ErrInvalidPhase
	}
	w := normalizeWord(word)
	if w == "" {
		return ErrEmptyWord
	}
	r := b.CurrentRound()
	if _, dup := r.topicsBy[clientID]; dup {
		return ErrAlreadySubmitted
	}
	r.topicsBy[clientID] = w
	return nil
}

// StartWriting closes topic submission, picks the most-submitted word
// as the round topic, and moves to the writing phase.
func (b *Battle) StartWriting() (string, error) {
	if b.phase != PhaseTopicsOpen {
		return "", ErrInvalidPhase
	}
	r := b.CurrentRound()
	counts := r.TopicCounts()
	if len(counts) == 0 {
		return "", ErrNoTopics
	}
	type wc struct {
		word  string
		count int
	}
	ranked := make([]wc, 0, len(counts))
	for w, c := range counts {
		ranked = append(ranked, wc{w, c})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].count != ranked[j].count {
			return ranked[i].count > ranked[j].count
		}
		return ranked[i].word < ranked[j].word // deterministic tie-break
	})
	r.topic = ranked[0].word
	b.phase = PhaseWriting
	return r.topic, nil
}

// SetVerse stores a battler's finished verse during the writing phase.
func (b *Battle) SetVerse(battler Battler, verse string) error {
	if b.phase != PhaseWriting {
		return ErrInvalidPhase
	}
	if !battler.Valid() {
		return ErrInvalidBattler
	}
	b.CurrentRound().verses[battler] = verse
	return nil
}

// StartPerformances moves to the first performance once both verses exist.
func (b *Battle) StartPerformances() error {
	if b.phase != PhaseWriting {
		return ErrInvalidPhase
	}
	r := b.CurrentRound()
	if r.verses[BattlerBlue] == "" || r.verses[BattlerRed] == "" {
		return ErrInvalidPhase
	}
	b.phase = PhasePerformingA
	return nil
}

// NextPerformance moves from the first performance to the second.
func (b *Battle) NextPerformance() error {
	if b.phase != PhasePerformingA {
		return ErrInvalidPhase
	}
	b.phase = PhasePerformingB
	return nil
}

// OpenVoting opens the audience vote after both performances.
func (b *Battle) OpenVoting() error {
	if b.phase != PhasePerformingB {
		return ErrInvalidPhase
	}
	b.phase = PhaseVoting
	return nil
}

// Vote records one vote per client per round.
func (b *Battle) Vote(clientID string, battler Battler) error {
	if b.phase != PhaseVoting {
		return ErrInvalidPhase
	}
	if !battler.Valid() {
		return ErrInvalidBattler
	}
	r := b.CurrentRound()
	if _, dup := r.votesBy[clientID]; dup {
		return ErrAlreadyVoted
	}
	r.votesBy[clientID] = battler
	return nil
}

// CloseRound tallies votes, stores the judge's commentary and either
// shows the round result or crowns the champion after the last round.
func (b *Battle) CloseRound(judgeCommentary string) (Battler, error) {
	if b.phase != PhaseVoting {
		return "", ErrInvalidPhase
	}
	r := b.CurrentRound()
	counts := r.VoteCounts()
	switch {
	case counts[BattlerBlue] > counts[BattlerRed]:
		r.roundWinner = BattlerBlue
	case counts[BattlerRed] > counts[BattlerBlue]:
		r.roundWinner = BattlerRed
	}
	r.judgeSay = judgeCommentary
	if len(b.rounds) >= b.totalRounds {
		b.phase = PhaseChampion
	} else {
		b.phase = PhaseRoundResult
	}
	return r.roundWinner, nil
}

// AddCrowdWord stores a live audience word for the crowd-scanner tool.
// Only the most recent 50 are kept.
func (b *Battle) AddCrowdWord(word string) error {
	w := normalizeWord(word)
	if w == "" {
		return ErrEmptyWord
	}
	b.crowdWords = append(b.crowdWords, w)
	if len(b.crowdWords) > 50 {
		b.crowdWords = b.crowdWords[len(b.crowdWords)-50:]
	}
	return nil
}

// RecentCrowdWords returns up to n most recent audience words, newest first.
func (b *Battle) RecentCrowdWords(n int) []string {
	out := make([]string, 0, n)
	for i := len(b.crowdWords) - 1; i >= 0 && len(out) < n; i-- {
		out = append(out, b.crowdWords[i])
	}
	return out
}

func normalizeWord(w string) string {
	w = strings.TrimSpace(strings.ToLower(w))
	if len(w) > 40 {
		w = w[:40]
	}
	return w
}
