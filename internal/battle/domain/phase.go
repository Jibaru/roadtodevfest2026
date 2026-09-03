package domain

// Phase is the battle state machine position.
type Phase string

const (
	PhaseIdle        Phase = "idle"
	PhaseTopicsOpen  Phase = "topics_open"
	PhaseWriting     Phase = "writing"
	PhasePerformingA Phase = "performing_a"
	PhasePerformingB Phase = "performing_b"
	PhaseVoting      Phase = "voting"
	PhaseRoundResult Phase = "round_result"
	PhaseChampion    Phase = "champion"
)

// Battler identifies one of the two competing agents.
type Battler string

const (
	BattlerGopher  Battler = "gopher"
	BattlerNullPtr Battler = "nullptr"
)

func (b Battler) Valid() bool {
	return b == BattlerGopher || b == BattlerNullPtr
}

// Opponent returns the other battler.
func (b Battler) Opponent() Battler {
	if b == BattlerGopher {
		return BattlerNullPtr
	}
	return BattlerGopher
}
