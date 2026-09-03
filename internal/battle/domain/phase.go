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
	BattlerBlue Battler = "blue"
	BattlerRed  Battler = "red"
)

func (b Battler) Valid() bool {
	return b == BattlerBlue || b == BattlerRed
}

// Opponent returns the other battler.
func (b Battler) Opponent() Battler {
	if b == BattlerBlue {
		return BattlerRed
	}
	return BattlerBlue
}
