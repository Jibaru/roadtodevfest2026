package domain

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

// VideoRepository is the persistence port. Implementations live in
// infra/persistence and must return domain errors. Same interface for
// memory and Postgres — swapping storage is one constructor line.
type VideoRepository interface {
	Create(ctx context.Context, v *Video) error
	ByID(ctx context.Context, id string) (*Video, error)
	ByYouTubeID(ctx context.Context, youtubeID string) (*Video, error)
	ListRecent(ctx context.Context, limit int) ([]*Video, error)
	UpdateMeta(ctx context.Context, id, title, thumbnailURL string, durationSec int) error
	MarkReady(ctx context.Context, id string, lyrics []LyricLine, language string) error
	MarkFailed(ctx context.Context, id string, message string) error
	Delete(ctx context.Context, id string) error
}

// NextID generates a new video ID. ID creation is a domain
// responsibility; repositories only store what they are given.
func NextID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand failing means the platform is broken
	}
	return hex.EncodeToString(b)
}
