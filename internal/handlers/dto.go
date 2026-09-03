package handlers

import "github.com/jibaru/rapbattle/internal/battle/domain"

// StartBattleRequest configures a new battle.
type StartBattleRequest struct {
	Rounds int `json:"rounds"`
}

// BattleResponse is the HTTP projection of the battle plus live info.
type BattleResponse struct {
	Battle        domain.BattleState `json:"battle"`
	AudienceCount int                `json:"audience_count"`
}

// ErrorResponse is the uniform error body.
type ErrorResponse struct {
	Error string `json:"error"`
}
