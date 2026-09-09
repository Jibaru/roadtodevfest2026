package domain

// SessionState is the exported, serializable projection of a Session:
// the data-mapper boundary used by the file repository for snapshots
// and by the realtime layer as the broadcast payload.
type SessionState struct {
	ID     string           `json:"id"`
	Phase  Phase            `json:"phase"`
	Rounds []RoundState     `json:"rounds"`
	Scores map[Reviewer]int `json:"scores"`
}

type RoundState struct {
	Number     int               `json:"number"`
	ReposBy    map[string]string `json:"repos_by,omitempty"`
	RepoCounts map[string]int    `json:"repo_counts,omitempty"`
	Repo       string            `json:"repo,omitempty"`
	Findings   []Finding         `json:"findings,omitempty"`
	VotesBy    map[string]string `json:"votes_by,omitempty"`
	VoteCounts map[string]int    `json:"vote_counts,omitempty"`
	Summary    string            `json:"summary,omitempty"`
}

// Snapshot projects the session into its serializable state.
func (s *Session) Snapshot() SessionState {
	st := SessionState{ID: s.id, Phase: s.phase, Scores: s.Scores()}
	for _, r := range s.rounds {
		st.Rounds = append(st.Rounds, RoundState{
			Number:     r.number,
			ReposBy:    copyMap(r.reposBy),
			RepoCounts: r.RepoCounts(),
			Repo:       r.repo,
			Findings:   r.Findings(),
			VotesBy:    copyMap(r.votesBy),
			VoteCounts: r.VoteCounts(),
			Summary:    r.summary,
		})
	}
	return st
}

// RestoreSession rebuilds a Session aggregate from a snapshot.
func RestoreSession(st SessionState) *Session {
	s := NewSession(st.ID)
	s.phase = st.Phase
	for _, rs := range st.Rounds {
		r := newRound(rs.Number)
		r.repo = rs.Repo
		r.summary = rs.Summary
		r.findings = append(r.findings, rs.Findings...)
		r.nextFinding = len(rs.Findings)
		for k, v := range rs.ReposBy {
			r.reposBy[k] = v
		}
		for k, v := range rs.VotesBy {
			r.votesBy[k] = v
		}
		s.rounds = append(s.rounds, r)
	}
	return s
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
