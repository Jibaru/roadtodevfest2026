package domain

// Phase is the review-session state machine position.
type Phase string

const (
	PhaseIdle      Phase = "idle"
	PhaseReposOpen Phase = "repos_open"
	PhaseReviewing Phase = "reviewing"
	PhaseResults   Phase = "results"
)

// Reviewer identifies one of the competing reviewer agents.
type Reviewer string

const (
	ReviewerBugs     Reviewer = "bugs"
	ReviewerSecurity Reviewer = "security"
	ReviewerSimplify Reviewer = "simplify"
)

// AllReviewers is the fixed reviewing crew, in display order.
var AllReviewers = []Reviewer{ReviewerBugs, ReviewerSecurity, ReviewerSimplify}

func (r Reviewer) Valid() bool {
	for _, v := range AllReviewers {
		if r == v {
			return true
		}
	}
	return false
}
