package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/jibaru/s1ngo/internal/pipeline"
	"github.com/jibaru/s1ngo/internal/realtime"
	"github.com/jibaru/s1ngo/internal/video/domain"
)

var fingerprintRe = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)

// Handlers holds the HTTP surface. Thin by design: validate, call the
// pipeline/repo, translate errors — no business logic.
type Handlers struct {
	pipe *pipeline.Service
	repo domain.VideoRepository
	hub  *realtime.Hub
	log  *slog.Logger
}

func New(pipe *pipeline.Service, repo domain.VideoRepository, hub *realtime.Hub, log *slog.Logger) *Handlers {
	return &Handlers{pipe: pipe, repo: repo, hub: hub, log: log}
}

func (h *Handlers) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handlers) ListVideos(w http.ResponseWriter, r *http.Request) {
	videos, err := h.repo.ListRecent(r.Context(), 100)
	if err != nil {
		h.writeError(w, err)
		return
	}
	fp := fingerprintHash(r)
	for _, v := range videos {
		v.Owned = fp != "" && v.OwnerFingerprintHash == fp
	}
	writeJSON(w, http.StatusOK, map[string]any{"videos": videos})
}

func (h *Handlers) GetVideo(w http.ResponseWriter, r *http.Request) {
	v, err := h.repo.ByID(r.Context(), r.PathValue("id"))
	if err != nil {
		h.writeError(w, err)
		return
	}
	fp := fingerprintHash(r)
	v.Owned = fp != "" && v.OwnerFingerprintHash == fp
	writeJSON(w, http.StatusOK, map[string]any{"video": v})
}

func (h *Handlers) CreateVideo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid JSON body"))
		return
	}
	v, err := h.pipe.Enqueue(r.Context(), req.URL, fingerprintHash(r))
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"video": v})
}

func (h *Handlers) DeleteVideo(w http.ResponseWriter, r *http.Request) {
	if err := h.pipe.Delete(r.Context(), r.PathValue("id"), fingerprintHash(r)); err != nil {
		h.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) WS(w http.ResponseWriter, r *http.Request) {
	h.hub.ServeWS(w, r)
}

// fingerprintHash hashes the browser's self-issued fingerprint header,
// mirroring the original s1ng owner mechanism.
func fingerprintHash(r *http.Request) string {
	raw := r.Header.Get("X-Fingerprint")
	if !fingerprintRe.MatchString(raw) {
		return ""
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// writeError maps domain errors to HTTP status codes.
func (h *Handlers) writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrVideoNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrInvalidYouTubeURL):
		status = http.StatusBadRequest
	case errors.Is(err, domain.ErrAlreadyQueued):
		status = http.StatusConflict
	case errors.Is(err, domain.ErrNotOwner):
		status = http.StatusForbidden
	default:
		h.log.Error("internal error", "error", err)
	}
	writeJSON(w, status, errBody(err.Error()))
}

func errBody(msg string) map[string]string { return map[string]string{"error": msg} }

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
