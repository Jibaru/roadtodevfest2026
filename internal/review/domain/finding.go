package domain

// Severity of a finding.
const (
	SeverityHigh   = "high"
	SeverityMedium = "medium"
	SeverityLow    = "low"
)

// Finding is one review observation produced by a reviewer agent.
// Exported fields: findings are produced outside the aggregate (by
// agents) and only gain identity when added to a round.
type Finding struct {
	ID         string   `json:"id"`
	Reviewer   Reviewer `json:"reviewer"`
	Severity   string   `json:"severity"`
	Title      string   `json:"title"`
	File       string   `json:"file,omitempty"`
	Detail     string   `json:"detail"`
	Suggestion string   `json:"suggestion,omitempty"`
}

func normalizeSeverity(s string) string {
	switch s {
	case SeverityHigh, SeverityMedium, SeverityLow:
		return s
	default:
		return SeverityMedium
	}
}

// severityRank orders findings for display: high first.
func severityRank(s string) int {
	switch s {
	case SeverityHigh:
		return 0
	case SeverityMedium:
		return 1
	default:
		return 2
	}
}
