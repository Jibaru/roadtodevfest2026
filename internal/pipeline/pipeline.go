// Package pipeline is the heart of s1n.go: a worker pool that turns
// YouTube URLs into karaoke, several videos at a time. The original
// s1ng processes one video synchronously inside the submit request;
// here every submission is queued instantly and N goroutine workers
// chew through the queue in parallel, streaming progress over
// WebSocket as they go.
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jibaru/s1ngo/internal/lines"
	"github.com/jibaru/s1ngo/internal/video/domain"
	"github.com/jibaru/s1ngo/internal/ytdlp"
)

const (
	fetchTimeout = 2 * time.Minute
	agentTimeout = 2 * time.Minute
	queueSize    = 64
)

// Agents is the ADK crew port: language detective, romanizer, translator.
type Agents interface {
	DetectLanguage(ctx context.Context, title, description string) (string, error)
	Romanize(ctx context.Context, language string, texts []string) ([]string, error)
	Translate(ctx context.Context, sourceLang string, texts []string) ([]string, error)
}

// Fetcher is the yt-dlp port.
type Fetcher interface {
	Fetch(ctx context.Context, youtubeID string, detectLang ytdlp.DetectLangFunc) (*ytdlp.Result, error)
}

// Broadcaster pushes live events to every connected browser.
type Broadcaster interface {
	Broadcast(eventType string, payload any)
}

type job struct {
	videoID   string
	youtubeID string
}

// Service owns the queue, the workers and the video lifecycle.
type Service struct {
	repo    domain.VideoRepository
	fetcher Fetcher
	agents  Agents
	cast    Broadcaster
	log     *slog.Logger

	jobs chan job

	mu       sync.Mutex
	inFlight map[string]bool // youtubeID -> queued/processing
}

// New starts `workers` goroutines consuming the queue.
func New(repo domain.VideoRepository, fetcher Fetcher, agents Agents, cast Broadcaster, workers int, log *slog.Logger) *Service {
	if workers < 1 {
		workers = 3
	}
	s := &Service{
		repo:     repo,
		fetcher:  fetcher,
		agents:   agents,
		cast:     cast,
		log:      log,
		jobs:     make(chan job, queueSize),
		inFlight: map[string]bool{},
	}
	for i := 0; i < workers; i++ {
		go s.worker(i)
	}
	log.Info("pipeline started", "workers", workers)
	return s
}

// Enqueue registers a video and queues it for processing. If the video
// already exists it is returned as-is (the UI navigates to it).
func (s *Service) Enqueue(ctx context.Context, url, ownerFingerprintHash string) (*domain.Video, error) {
	youtubeID, err := domain.ParseYouTubeID(url)
	if err != nil {
		return nil, err
	}

	if existing, err := s.repo.ByYouTubeID(ctx, youtubeID); err == nil {
		if existing.Status != domain.StatusFailed {
			return existing, nil
		}
		// Failed videos are retried on resubmission (YouTube's bot
		// checks are intermittent — the next attempt often works).
		s.mu.Lock()
		if s.inFlight[youtubeID] {
			s.mu.Unlock()
			return existing, nil
		}
		s.inFlight[youtubeID] = true
		s.mu.Unlock()
		if err := s.repo.MarkProcessing(ctx, existing.ID); err != nil {
			s.release(youtubeID)
			return nil, err
		}
		existing.Status = domain.StatusProcessing
		existing.ErrorMessage = ""
		s.upsert(existing)
		s.progress(existing.ID, "retrying")
		s.jobs <- job{videoID: existing.ID, youtubeID: youtubeID}
		return existing, nil
	}

	s.mu.Lock()
	if s.inFlight[youtubeID] {
		s.mu.Unlock()
		return nil, domain.ErrAlreadyQueued
	}
	s.inFlight[youtubeID] = true
	s.mu.Unlock()

	v := &domain.Video{
		ID:                   domain.NextID(),
		YouTubeID:            youtubeID,
		Title:                youtubeID, // real title arrives with metadata
		ThumbnailURL:         "https://i.ytimg.com/vi/" + youtubeID + "/hqdefault.jpg",
		Status:               domain.StatusProcessing,
		OwnerFingerprintHash: ownerFingerprintHash,
		CreatedAt:            time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, v); err != nil {
		s.release(youtubeID)
		return nil, err
	}
	s.upsert(v)
	s.progress(v.ID, "queued")
	s.jobs <- job{videoID: v.ID, youtubeID: youtubeID}
	return v, nil
}

// Delete removes a video if the requester's fingerprint matches.
func (s *Service) Delete(ctx context.Context, id, fingerprintHash string) error {
	v, err := s.repo.ByID(ctx, id)
	if err != nil {
		return err
	}
	if v.OwnerFingerprintHash == "" || fingerprintHash == "" || v.OwnerFingerprintHash != fingerprintHash {
		return domain.ErrNotOwner
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.cast.Broadcast("video_deleted", map[string]string{"id": id})
	return nil
}

func (s *Service) worker(n int) {
	for j := range s.jobs {
		s.process(j)
		s.release(j.youtubeID)
	}
	_ = n
}

// process runs one video through the pipeline. Every failure becomes a
// visible "failed" card — the queue keeps moving.
func (s *Service) process(j job) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout+2*agentTimeout)
	defer cancel()

	fail := func(err error) {
		s.log.Error("video failed", "video", j.videoID, "youtube", j.youtubeID, "error", err)
		_ = s.repo.MarkFailed(context.Background(), j.videoID, err.Error())
		s.upsertByID(j.videoID)
	}

	// Stage 1: metadata + subtitles (two-phase yt-dlp). The language
	// detective agent is injected as the detection callback.
	s.progress(j.videoID, "fetching subtitles")
	fctx, fcancel := context.WithTimeout(ctx, fetchTimeout)
	res, err := s.fetcher.Fetch(fctx, j.youtubeID, func(dctx context.Context, title, desc string) string {
		s.progress(j.videoID, "detecting language")
		lang, derr := s.agents.DetectLanguage(dctx, title, desc)
		if derr != nil {
			s.log.Warn("language detective unavailable, falling back to title script",
				"video", j.videoID, "error", derr)
			return ""
		}
		return lang
	})
	fcancel()
	if err != nil {
		fail(err)
		return
	}

	// Real metadata replaces the placeholder immediately.
	if err := s.repo.UpdateMeta(context.Background(), j.videoID, res.Title, res.ThumbnailURL, res.DurationSec); err == nil {
		s.upsertByID(j.videoID)
	}

	cues := lines.CleanCues(res.Cues)
	if res.SubtitleLang == "" || len(cues) == 0 {
		fail(domain.ErrNoSubtitles)
		return
	}

	texts := make([]string, len(cues))
	for i, c := range cues {
		texts[i] = c.Text
	}

	// Stage 2: romanize (ja/ko only). If the agent fails we keep the
	// original script — degraded but alive.
	var displays []string
	if lines.NeedsRomanization(res.SubtitleLang) {
		s.progress(j.videoID, fmt.Sprintf("romanizing %d lines", len(texts)))
		actx, acancel := context.WithTimeout(ctx, agentTimeout)
		displays, err = s.agents.Romanize(actx, res.SubtitleLang, texts)
		acancel()
		if err != nil {
			s.log.Warn("romanizer failed, keeping original script", "video", j.videoID, "error", err)
			displays = nil
		}
	}

	// Stage 3: translate. Optional flourish — empty on failure.
	s.progress(j.videoID, fmt.Sprintf("translating %d lines", len(texts)))
	tctx, tcancel := context.WithTimeout(ctx, agentTimeout)
	translations, err := s.agents.Translate(tctx, res.SubtitleLang, texts)
	tcancel()
	if err != nil {
		s.log.Warn("translator failed, shipping without translations", "video", j.videoID, "error", err)
		translations = nil
	}

	// Stage 4: word-level timing and done.
	s.progress(j.videoID, "building karaoke lines")
	built := lines.Build(cues, displays, translations)
	if err := s.repo.MarkReady(context.Background(), j.videoID, built, res.SubtitleLang); err != nil {
		fail(err)
		return
	}
	s.progress(j.videoID, "ready")
	s.upsertByID(j.videoID)
	s.log.Info("video ready", "video", j.videoID, "youtube", j.youtubeID,
		"lang", res.SubtitleLang, "lines", len(built), "auto_subs", res.IsAuto)
}

func (s *Service) release(youtubeID string) {
	s.mu.Lock()
	delete(s.inFlight, youtubeID)
	s.mu.Unlock()
}

func (s *Service) progress(videoID, stage string) {
	s.cast.Broadcast("progress", map[string]string{"id": videoID, "stage": stage})
}

func (s *Service) upsert(v *domain.Video) {
	s.cast.Broadcast("video", v)
}

func (s *Service) upsertByID(id string) {
	if v, err := s.repo.ByID(context.Background(), id); err == nil {
		s.upsert(v)
	} else if !errors.Is(err, domain.ErrVideoNotFound) {
		s.log.Error("upsert lookup", "video", id, "error", err)
	}
}
