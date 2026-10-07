package domain

import "context"

// SubtitleCue is one subtitle entry with millisecond timing, as fetched
// from the video platform.
type SubtitleCue struct {
	StartMs int
	EndMs   int
	Text    string
}

// SubtitleResult is everything fetching one video's metadata and
// subtitles produces.
type SubtitleResult struct {
	Title            string
	DurationSec      int
	ThumbnailURL     string
	DeclaredLanguage string
	Description      string
	// SubtitleLang is the language of the chosen track ("" if none found).
	SubtitleLang string
	// IsAuto is true when the track is auto-generated, not human-authored.
	IsAuto bool
	Cues   []SubtitleCue
}

// LanguageDetectFunc resolves the song's actual lyrics language from
// its title and description (an LLM agent in production). It returns
// "" when unknown, letting the caller fall back to script analysis.
type LanguageDetectFunc func(ctx context.Context, title, description string) string
