package domain

// BattleState is the exported, serializable projection of a Battle.
// It is the data-mapper boundary: used by the file repository for
// snapshots and by the realtime layer as the broadcast payload.
type BattleState struct {
	ID          string       `json:"id"`
	Phase       Phase        `json:"phase"`
	TotalRounds int          `json:"total_rounds"`
	Rounds      []RoundState `json:"rounds"`
	Scores      map[Battler]int `json:"scores"`
	Champion    Battler      `json:"champion,omitempty"`
	CrowdWords  []string     `json:"crowd_words,omitempty"`
}

type RoundState struct {
	Number          int                `json:"number"`
	Topic           string             `json:"topic,omitempty"`
	TopicsBy        map[string]string  `json:"topics_by,omitempty"`
	TopicCounts     map[string]int     `json:"topic_counts,omitempty"`
	Verses          map[Battler]string `json:"verses,omitempty"`
	VotesBy         map[string]Battler `json:"votes_by,omitempty"`
	VoteCounts      map[Battler]int    `json:"vote_counts"`
	JudgeCommentary string             `json:"judge_commentary,omitempty"`
	Winner          Battler            `json:"winner,omitempty"`
}

// Snapshot projects the battle into its serializable state.
func (b *Battle) Snapshot() BattleState {
	s := BattleState{
		ID:          b.id,
		Phase:       b.phase,
		TotalRounds: b.totalRounds,
		Scores:      b.Scores(),
		Champion:    b.Champion(),
		CrowdWords:  append([]string(nil), b.crowdWords...),
	}
	for _, r := range b.rounds {
		rs := RoundState{
			Number:          r.number,
			Topic:           r.topic,
			TopicsBy:        copyMap(r.topicsBy),
			TopicCounts:     r.TopicCounts(),
			Verses:          copyMap(r.verses),
			VotesBy:         copyMap(r.votesBy),
			VoteCounts:      r.VoteCounts(),
			JudgeCommentary: r.judgeSay,
			Winner:          r.roundWinner,
		}
		s.Rounds = append(s.Rounds, rs)
	}
	return s
}

// RestoreBattle rebuilds a Battle aggregate from a snapshot.
func RestoreBattle(s BattleState) *Battle {
	b := NewBattle(s.ID, s.TotalRounds)
	b.phase = s.Phase
	b.crowdWords = append([]string(nil), s.CrowdWords...)
	for _, rs := range s.Rounds {
		r := newRound(rs.Number)
		r.topic = rs.Topic
		r.judgeSay = rs.JudgeCommentary
		r.roundWinner = rs.Winner
		for k, v := range rs.TopicsBy {
			r.topicsBy[k] = v
		}
		for k, v := range rs.Verses {
			r.verses[k] = v
		}
		for k, v := range rs.VotesBy {
			r.votesBy[k] = v
		}
		b.rounds = append(b.rounds, r)
	}
	return b
}

func copyMap[K comparable, V any](m map[K]V) map[K]V {
	if len(m) == 0 {
		return nil
	}
	out := make(map[K]V, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
