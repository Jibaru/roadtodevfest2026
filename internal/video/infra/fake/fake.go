// Package fake provides no-API rehearsal doubles: run the whole show
// offline with FAKE_AGENTS=1. The fake subtitle source serves an
// embedded three-line "song" so processing works without yt-dlp,
// network, or an OpenAI key.
package fake

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jibaru/s1ngo/internal/video/domain"
)

// Agents fakes the three lyrics agents (service.LyricsAgents).
type Agents struct {
	Delay time.Duration
}

func (a *Agents) DetectLanguage(ctx context.Context, title, _ string) (string, error) {
	a.wait(ctx)
	return domain.DetectLangFromTitle(title), nil
}

func (a *Agents) Romanize(ctx context.Context, _ string, texts []string) ([]string, error) {
	a.wait(ctx)
	out := make([]string, len(texts))
	for i, t := range texts {
		out[i] = "romaji( " + t + " )"
	}
	return out, nil
}

func (a *Agents) Translate(ctx context.Context, _ string, texts []string) ([]string, error) {
	a.wait(ctx)
	out := make([]string, len(texts))
	for i, t := range texts {
		out[i] = "[es] " + t
	}
	return out, nil
}

func (a *Agents) wait(ctx context.Context) {
	select {
	case <-time.After(a.Delay):
	case <-ctx.Done():
	}
}

// SubtitleSource fakes yt-dlp with an embedded three-line song.
type SubtitleSource struct{}

func (SubtitleSource) Fetch(_ context.Context, youtubeID string, _ domain.LanguageDetectFunc) (*domain.SubtitleResult, error) {
	cues := []domain.SubtitleCue{
		{StartMs: 0, EndMs: 2500, Text: "hello from the fake song"},
		{StartMs: 2500, EndMs: 5200, Text: "three lines is all we need"},
		{StartMs: 5200, EndMs: 8000, Text: "to rehearse the whole pipeline"},
	}
	title := "Fake Song (" + strings.ToUpper(youtubeID[:4]) + " rehearsal)"
	return &domain.SubtitleResult{
		Title:        title,
		DurationSec:  8,
		ThumbnailURL: fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", youtubeID),
		SubtitleLang: "en",
		IsAuto:       false,
		Cues:         cues,
	}, nil
}
