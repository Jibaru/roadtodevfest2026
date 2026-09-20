package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/jibaru/s1ngo/internal/video/domain"
)

// VideoRepository keeps videos in memory. Great for local rehearsals;
// production uses the Postgres implementation of the same interface.
type VideoRepository struct {
	mu     sync.RWMutex
	byID   map[string]*domain.Video
}

func NewVideoRepository() *VideoRepository {
	return &VideoRepository{byID: map[string]*domain.Video{}}
}

func (r *VideoRepository) Create(_ context.Context, v *domain.Video) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *v
	r.byID[v.ID] = &cp
	return nil
}

func (r *VideoRepository) ByID(_ context.Context, id string) (*domain.Video, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if v, ok := r.byID[id]; ok {
		cp := *v
		return &cp, nil
	}
	return nil, domain.ErrVideoNotFound
}

func (r *VideoRepository) ByYouTubeID(_ context.Context, youtubeID string) (*domain.Video, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, v := range r.byID {
		if v.YouTubeID == youtubeID {
			cp := *v
			return &cp, nil
		}
	}
	return nil, domain.ErrVideoNotFound
}

func (r *VideoRepository) ListRecent(_ context.Context, limit int) ([]*domain.Video, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*domain.Video, 0, len(r.byID))
	for _, v := range r.byID {
		cp := *v
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *VideoRepository) UpdateMeta(_ context.Context, id, title, thumbnailURL string, durationSec int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.byID[id]
	if !ok {
		return domain.ErrVideoNotFound
	}
	v.Title = title
	v.ThumbnailURL = thumbnailURL
	v.DurationSec = durationSec
	return nil
}

func (r *VideoRepository) MarkProcessing(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.byID[id]
	if !ok {
		return domain.ErrVideoNotFound
	}
	v.Status = domain.StatusProcessing
	v.ErrorMessage = ""
	return nil
}

func (r *VideoRepository) MarkReady(_ context.Context, id string, lyrics []domain.LyricLine, language string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.byID[id]
	if !ok {
		return domain.ErrVideoNotFound
	}
	v.Status = domain.StatusReady
	v.Lyrics = lyrics
	v.Language = language
	v.ErrorMessage = ""
	return nil
}

func (r *VideoRepository) MarkFailed(_ context.Context, id string, message string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.byID[id]
	if !ok {
		return domain.ErrVideoNotFound
	}
	v.Status = domain.StatusFailed
	v.ErrorMessage = message
	return nil
}

func (r *VideoRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[id]; !ok {
		return domain.ErrVideoNotFound
	}
	delete(r.byID, id)
	return nil
}
