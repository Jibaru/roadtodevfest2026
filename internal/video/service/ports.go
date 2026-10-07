package service

import (
	"context"

	"github.com/jibaru/s1ngo/internal/video/domain"
)

// LyricsAgents is the AI port: three focused jobs the Go ecosystem has
// no libraries for. Implemented in infra/agents (OpenAI via ADK, plus
// an offline fake for rehearsals).
type LyricsAgents interface {
	// DetectLanguage returns ja/ko/es/en from title+description, or ""
	// when unknown — the caller falls back to script analysis.
	DetectLanguage(ctx context.Context, title, description string) (string, error)
	// Romanize transliterates ja/ko lyric lines to Latin script,
	// leaving embedded Latin words (English/Spanish) untouched.
	Romanize(ctx context.Context, language string, texts []string) ([]string, error)
	// Translate renders each line in Spanish (English when the song is
	// already Spanish).
	Translate(ctx context.Context, sourceLang string, texts []string) ([]string, error)
}

// SubtitleSource fetches a video's metadata and subtitle track.
// Implemented in infra/youtube (yt-dlp) and infra/agents (fake).
type SubtitleSource interface {
	Fetch(ctx context.Context, youtubeID string, detect domain.LanguageDetectFunc) (*domain.SubtitleResult, error)
}

// Broadcaster pushes live events to every connected browser.
// Implemented by the realtime hub.
type Broadcaster interface {
	Broadcast(eventType string, payload any)
}
