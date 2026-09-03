package domain

import "errors"

// Domain errors. Infrastructure implementations must return these,
// never their own driver-specific errors.
var (
	ErrBattleNotFound   = errors.New("battle not found")
	ErrInvalidPhase     = errors.New("action not allowed in current phase")
	ErrAlreadySubmitted = errors.New("client already submitted a topic this round")
	ErrAlreadyVoted     = errors.New("client already voted this round")
	ErrInvalidBattler   = errors.New("unknown battler")
	ErrNoTopics         = errors.New("no topics were submitted")
	ErrEmptyWord        = errors.New("word must not be empty")
	ErrVerseNotCached   = errors.New("no cached verse for topic")
)
