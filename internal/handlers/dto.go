package handlers

import "github.com/jibaru/agentarena/internal/review/domain"

// SessionResponse is the HTTP projection of the session plus live info.
type SessionResponse struct {
	Session       domain.SessionState `json:"session"`
	AudienceCount int                 `json:"audience_count"`
}

// ErrorResponse is the uniform error body.
type ErrorResponse struct {
	Error string `json:"error"`
}
