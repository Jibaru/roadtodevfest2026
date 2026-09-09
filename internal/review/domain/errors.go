package domain

import "errors"

// Domain errors. Infrastructure implementations must return these,
// never their own driver-specific errors.
var (
	ErrSessionNotFound  = errors.New("session not found")
	ErrInvalidPhase     = errors.New("action not allowed in current phase")
	ErrAlreadySubmitted = errors.New("client already submitted a repo this round")
	ErrAlreadyVoted     = errors.New("client already voted this round")
	ErrInvalidReviewer  = errors.New("unknown reviewer")
	ErrInvalidRepoURL   = errors.New("not a valid public GitHub repo (use github.com/owner/repo)")
	ErrNoRepos          = errors.New("no repositories were submitted")
	ErrFindingNotFound  = errors.New("finding not found")
)
