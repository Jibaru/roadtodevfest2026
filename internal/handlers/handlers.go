package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jibaru/agentarena/internal/battle/domain"
	"github.com/jibaru/agentarena/internal/battle/service"
	"github.com/jibaru/agentarena/internal/realtime"
)

const clientCookie = "rapbattle_client"

// Handlers holds the HTTP surface. Thin by design: validate, call the
// service, translate errors — no business logic.
type Handlers struct {
	svc            *service.BattleService
	hub            *realtime.Hub
	presenterToken string
	log            *slog.Logger
}

func New(svc *service.BattleService, hub *realtime.Hub, presenterToken string, log *slog.Logger) *Handlers {
	return &Handlers{svc: svc, hub: hub, presenterToken: presenterToken, log: log}
}

func (h *Handlers) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handlers) StartBattle(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "invalid presenter token"})
		return
	}
	var req StartBattleRequest
	_ = json.NewDecoder(r.Body).Decode(&req) // empty body -> defaults
	state, err := h.svc.StartBattle(r.Context(), req.Rounds)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, h.response(state))
}

func (h *Handlers) Advance(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "invalid presenter token"})
		return
	}
	state, err := h.svc.Advance(r.Context())
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.response(state))
}

func (h *Handlers) Reset(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "invalid presenter token"})
		return
	}
	if err := h.svc.Reset(r.Context()); err != nil {
		h.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) CurrentBattle(w http.ResponseWriter, r *http.Request) {
	state, err := h.svc.State(r.Context())
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.response(state))
}

// AudienceWS upgrades an audience connection, identifying the client
// by a cookie so topic submissions and votes deduplicate.
func (h *Handlers) AudienceWS(w http.ResponseWriter, r *http.Request) {
	clientID := h.ensureClientID(w, r)
	h.hub.ServeWS(w, r, clientID, realtime.RoomAudience)
}

// StageWS upgrades the presenter connection (token-gated).
func (h *Handlers) StageWS(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("token") != h.presenterToken {
		http.Error(w, "invalid presenter token", http.StatusUnauthorized)
		return
	}
	h.hub.ServeWS(w, r, "stage", realtime.RoomStage)
}

func (h *Handlers) authorized(r *http.Request) bool {
	return r.Header.Get("X-Presenter-Token") == h.presenterToken ||
		r.URL.Query().Get("token") == h.presenterToken
}

func (h *Handlers) ensureClientID(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(clientCookie); err == nil && c.Value != "" {
		return c.Value
	}
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{
		Name:     clientCookie,
		Value:    id,
		Path:     "/",
		MaxAge:   60 * 60 * 6,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return id
}

func (h *Handlers) response(state domain.BattleState) BattleResponse {
	return BattleResponse{Battle: state, AudienceCount: h.hub.AudienceCount()}
}

// writeError maps domain errors to HTTP status codes.
func (h *Handlers) writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrBattleNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrInvalidPhase),
		errors.Is(err, domain.ErrNoTopics),
		errors.Is(err, domain.ErrEmptyWord),
		errors.Is(err, domain.ErrInvalidBattler):
		status = http.StatusConflict
	case errors.Is(err, domain.ErrAlreadyVoted),
		errors.Is(err, domain.ErrAlreadySubmitted):
		status = http.StatusTooManyRequests
	default:
		h.log.Error("internal error", "error", err)
	}
	writeJSON(w, status, ErrorResponse{Error: err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
