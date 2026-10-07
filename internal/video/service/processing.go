// Package service holds the business logic: a worker pool that turns
// YouTube URLs into karaoke, several videos at a time. Submissions are
// queued instantly and N goroutine workers drain the queue, streaming
// per-stage progress over WebSocket as they go.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jibaru/s1ngo/internal/video/domain"
)

const (
	fetchTimeout = 2 * time.Minute
	agentTimeout = 2 * time.Minute
	queueSize    = 64
)

type job struct {
	videoID   string
	youtubeID string
}

// ProcessingService owns the queue, the workers and the video lifecycle.
type ProcessingService struct {
	videos    domain.VideoRepository
	subtitles SubtitleSource
	agents    LyricsAgents
	events    Broadcaster
	log       *slog.Logger

	jobs chan job

	mu       sync.Mutex
	inFlight map[string]bool // youtubeID -> queued or processing
}

// NewProcessingService starts `workers` goroutines consuming the queue.
func NewProcessingService(
	videos domain.VideoRepository,
	subtitles SubtitleSource,
	agents LyricsAgents,
	events Broadcaster,
	workers int,
	log *slog.Logger,
) *ProcessingService {
	if workers < 1 {
		workers = 3
	}
	s := &ProcessingService{
		videos:    videos,
		subtitles: subtitles,
		agents:    agents,
		events:    events,
		log:       log,
		jobs:      make(chan job, queueSize),
		inFlight:  map[string]bool{},
	}
	for i := 0; i < workers; i++ {
		go s.worker()
	}
	log.Info("processing service started", "workers", workers)
	return s
}

// Enqueue registers a video and queues it for processing. If the video
// already exists it is returned as-is (the UI navigates to it); a
// failed video is retried.
func (s *ProcessingService) Enqueue(ctx context.Context, url, ownerFingerprintHash string) (*domain.Video, error) {
	youtubeID, err := domain.ParseYouTubeID(url)
	if err != nil {
		return nil, err
	}

	if existing, err := s.videos.ByYouTubeID(ctx, youtubeID); err == nil {
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
		if err := s.videos.MarkProcessing(ctx, existing.ID); err != nil {
			s.release(youtubeID)
			return nil, err
		}
		existing.Status = domain.StatusProcessing
		existing.ErrorMessage = ""
		s.broadcastVideo(existing)
		s.broadcastStage(existing.ID, "retrying")
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
	if err := s.videos.Create(ctx, v); err != nil {
		s.release(youtubeID)
		return nil, err
	}
	s.broadcastVideo(v)
	s.broadcastStage(v.ID, "queued")
	s.jobs <- job{videoID: v.ID, youtubeID: youtubeID}
	return v, nil
}

// Delete removes a video if the requester's fingerprint matches.
func (s *ProcessingService) Delete(ctx context.Context, id, fingerprintHash string) error {
	v, err := s.videos.ByID(ctx, id)
	if err != nil {
		return err
	}
	if v.OwnerFingerprintHash == "" || fingerprintHash == "" || v.OwnerFingerprintHash != fingerprintHash {
		return domain.ErrNotOwner
	}
	if err := s.videos.Delete(ctx, id); err != nil {
		return err
	}
	s.events.Broadcast("video_deleted", map[string]string{"id": id})
	return nil
}

func (s *ProcessingService) worker() {
	for j := range s.jobs {
		s.process(j)
		s.release(j.youtubeID)
	}
}

// process runs one video through the stages. Every failure becomes a
// visible "failed" card — the queue keeps moving.
func (s *ProcessingService) process(j job) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout+2*agentTimeout)
	defer cancel()

	fail := func(err error) {
		s.log.Error("video failed", "video", j.videoID, "youtube", j.youtubeID, "error", err)
		_ = s.videos.MarkFailed(context.Background(), j.videoID, err.Error())
		s.broadcastVideoByID(j.videoID)
	}

	// Stage 1: metadata + subtitles (two-phase yt-dlp). The language
	// detection agent is injected as the callback.
	s.broadcastStage(j.videoID, "fetching subtitles")
	fctx, fcancel := context.WithTimeout(ctx, fetchTimeout)
	result, err := s.subtitles.Fetch(fctx, j.youtubeID, func(dctx context.Context, title, desc string) string {
		s.broadcastStage(j.videoID, "detecting language")
		lang, derr := s.agents.DetectLanguage(dctx, title, desc)
		if derr != nil {
			s.log.Warn("language detection unavailable, falling back to title script",
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
	if err := s.videos.UpdateMeta(context.Background(), j.videoID, result.Title, result.ThumbnailURL, result.DurationSec); err == nil {
		s.broadcastVideoByID(j.videoID)
	}

	cues := domain.CleanCues(result.Cues)
	if result.SubtitleLang == "" || len(cues) == 0 {
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
	if domain.NeedsRomanization(result.SubtitleLang) {
		s.broadcastStage(j.videoID, fmt.Sprintf("romanizing %d lines", len(texts)))
		actx, acancel := context.WithTimeout(ctx, agentTimeout)
		displays, err = s.agents.Romanize(actx, result.SubtitleLang, texts)
		acancel()
		if err != nil {
			s.log.Warn("romanizer failed, keeping original script", "video", j.videoID, "error", err)
			displays = nil
		}
	}

	// Stage 3: translate. Optional flourish — empty on failure.
	s.broadcastStage(j.videoID, fmt.Sprintf("translating %d lines", len(texts)))
	tctx, tcancel := context.WithTimeout(ctx, agentTimeout)
	translations, err := s.agents.Translate(tctx, result.SubtitleLang, texts)
	tcancel()
	if err != nil {
		s.log.Warn("translator failed, shipping without translations", "video", j.videoID, "error", err)
		translations = nil
	}

	// Stage 4: word-level timing and done.
	s.broadcastStage(j.videoID, "building karaoke lines")
	built := domain.BuildLyricLines(cues, displays, translations)
	if err := s.videos.MarkReady(context.Background(), j.videoID, built, result.SubtitleLang); err != nil {
		fail(err)
		return
	}
	s.broadcastStage(j.videoID, "ready")
	s.broadcastVideoByID(j.videoID)
	s.log.Info("video ready", "video", j.videoID, "youtube", j.youtubeID,
		"lang", result.SubtitleLang, "lines", len(built), "auto_subs", result.IsAuto)
}

func (s *ProcessingService) release(youtubeID string) {
	s.mu.Lock()
	delete(s.inFlight, youtubeID)
	s.mu.Unlock()
}

func (s *ProcessingService) broadcastStage(videoID, stage string) {
	s.events.Broadcast("progress", map[string]string{"id": videoID, "stage": stage})
}

func (s *ProcessingService) broadcastVideo(v *domain.Video) {
	s.events.Broadcast("video", v)
}

func (s *ProcessingService) broadcastVideoByID(id string) {
	if v, err := s.videos.ByID(context.Background(), id); err == nil {
		s.broadcastVideo(v)
	} else if !errors.Is(err, domain.ErrVideoNotFound) {
		s.log.Error("broadcast lookup", "video", id, "error", err)
	}
}
