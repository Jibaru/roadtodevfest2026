package domain

import "time"

// Status of a video in the processing pipeline.
type Status string

const (
	StatusProcessing Status = "processing"
	StatusReady      Status = "ready"
	StatusFailed     Status = "failed"
)

// LyricWord is one timed token of a lyric line.
type LyricWord struct {
	Text    string `json:"text"`
	StartMs int    `json:"startMs"`
	EndMs   int    `json:"endMs"`
}

// LyricLine is one karaoke line with word-level timing. Translation is
// s1n.go's addition over the original: a per-line translation produced
// by the translator agent.
type LyricLine struct {
	StartMs     int         `json:"startMs"`
	EndMs       int         `json:"endMs"`
	Words       []LyricWord `json:"words"`
	Translation string      `json:"translation,omitempty"`
}

// Video is the persisted record for one processed YouTube video.
type Video struct {
	ID                   string      `json:"id"`
	YouTubeID            string      `json:"youtubeId"`
	Title                string      `json:"title"`
	ThumbnailURL         string      `json:"thumbnailUrl"`
	DurationSec          int         `json:"durationSec"`
	Language             string      `json:"language,omitempty"`
	Status               Status      `json:"status"`
	ErrorMessage         string      `json:"errorMessage,omitempty"`
	Lyrics               []LyricLine `json:"lyrics,omitempty"`
	OwnerFingerprintHash string      `json:"-"`
	// Owned is set per-request when serializing for a client whose
	// fingerprint matches; it is never persisted.
	Owned     bool      `json:"owned,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}
