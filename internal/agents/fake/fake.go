// Package fake provides no-API rehearsal doubles: run the whole show
// offline with FAKE_AGENTS=1. The fake fetcher serves an embedded
// bilingual "song" so processing works without yt-dlp or network.
package fake

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jibaru/s1ngo/internal/ytdlp"
)

// Crew fakes the three agents.
type Crew struct {
	Delay time.Duration
}

func (c *Crew) DetectLanguage(ctx context.Context, title, _ string) (string, error) {
	c.wait(ctx)
	return ytdlp.DetectLangFromTitle(title), nil
}

func (c *Crew) Romanize(ctx context.Context, _ string, texts []string) ([]string, error) {
	c.wait(ctx)
	out := make([]string, len(texts))
	for i, t := range texts {
		out[i] = "romaji( " + t + " )"
	}
	return out, nil
}

func (c *Crew) Translate(ctx context.Context, _ string, texts []string) ([]string, error) {
	c.wait(ctx)
	out := make([]string, len(texts))
	for i, t := range texts {
		out[i] = "[es] " + t
	}
	return out, nil
}

func (c *Crew) wait(ctx context.Context) {
	select {
	case <-time.After(c.Delay):
	case <-ctx.Done():
	}
}

// Fetcher fakes yt-dlp with an embedded three-line song.
type Fetcher struct{}

func (Fetcher) Fetch(_ context.Context, youtubeID string, _ ytdlp.DetectLangFunc) (*ytdlp.Result, error) {
	cues := []ytdlp.Cue{
		{StartMs: 0, EndMs: 2500, Text: "hello from the fake song"},
		{StartMs: 2500, EndMs: 5200, Text: "three lines is all we need"},
		{StartMs: 5200, EndMs: 8000, Text: "to rehearse the whole pipeline"},
	}
	title := "Fake Song (" + strings.ToUpper(youtubeID[:4]) + " rehearsal)"
	return &ytdlp.Result{
		Title:        title,
		DurationSec:  8,
		ThumbnailURL: fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", youtubeID),
		SubtitleLang: "en",
		IsAuto:       false,
		Cues:         cues,
	}, nil
}
