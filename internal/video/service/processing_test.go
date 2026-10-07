package service_test

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jibaru/s1ngo/internal/video/domain"
	"github.com/jibaru/s1ngo/internal/video/infra/fake"
	"github.com/jibaru/s1ngo/internal/video/infra/persistence/memory"
	"github.com/jibaru/s1ngo/internal/video/service"
)

type failingSource struct{ err error }

func (f failingSource) Fetch(context.Context, string, domain.LanguageDetectFunc) (*domain.SubtitleResult, error) {
	return nil, f.err
}

type noSubsSource struct{}

func (noSubsSource) Fetch(_ context.Context, id string, _ domain.LanguageDetectFunc) (*domain.SubtitleResult, error) {
	return &domain.SubtitleResult{Title: "No Subs " + id, ThumbnailURL: "t", DurationSec: 10}, nil
}

type castRecorder struct {
	mu     sync.Mutex
	events []string
}

func (c *castRecorder) Broadcast(eventType string, _ any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, eventType)
}

func newService(source service.SubtitleSource, workers int) (*service.ProcessingService, domain.VideoRepository) {
	repo := memory.NewVideoRepository()
	svc := service.NewProcessingService(repo, source, &fake.Agents{Delay: 10 * time.Millisecond},
		&castRecorder{}, workers, slog.New(slog.DiscardHandler))
	return svc, repo
}

func waitStatus(t *testing.T, repo domain.VideoRepository, id string, status domain.Status) *domain.Video {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		v, err := repo.ByID(context.Background(), id)
		require.NoError(t, err)
		if v.Status == status {
			return v
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for status %s", status)
	return nil
}

func TestProcessesVideo(t *testing.T) {
	svc, repo := newService(fake.SubtitleSource{}, 2)
	ctx := context.Background()

	v, err := svc.Enqueue(ctx, "https://youtu.be/dQw4w9WgXcQ", "fp-hash")
	require.NoError(t, err)
	assert.Equal(t, domain.StatusProcessing, v.Status)

	ready := waitStatus(t, repo, v.ID, domain.StatusReady)
	assert.Equal(t, "en", ready.Language)
	require.Len(t, ready.Lyrics, 3, "fake song has 3 lines")
	assert.NotEmpty(t, ready.Lyrics[0].Words)
	assert.Contains(t, ready.Lyrics[0].Translation, "[es]", "translator agent ran")
	assert.Contains(t, ready.Title, "Fake Song", "metadata updated after fetch")
}

func TestParallelism(t *testing.T) {
	svc, repo := newService(fake.SubtitleSource{}, 4)
	ctx := context.Background()

	ids := []string{"AAAAAAAAAAA", "BBBBBBBBBBB", "CCCCCCCCCCC", "DDDDDDDDDDD"}
	var videos []*domain.Video
	for _, y := range ids {
		v, err := svc.Enqueue(ctx, y, "")
		require.NoError(t, err)
		videos = append(videos, v)
	}
	for _, v := range videos {
		waitStatus(t, repo, v.ID, domain.StatusReady)
	}
}

func TestEnqueueDeduplicates(t *testing.T) {
	svc, repo := newService(fake.SubtitleSource{}, 1)
	ctx := context.Background()

	v1, err := svc.Enqueue(ctx, "dQw4w9WgXcQ", "")
	require.NoError(t, err)
	waitStatus(t, repo, v1.ID, domain.StatusReady)

	// Same video again → returns the existing record, no new job.
	v2, err := svc.Enqueue(ctx, "https://youtu.be/dQw4w9WgXcQ", "")
	require.NoError(t, err)
	assert.Equal(t, v1.ID, v2.ID)

	_, err = svc.Enqueue(ctx, "not a url", "")
	assert.ErrorIs(t, err, domain.ErrInvalidYouTubeURL)
}

func TestFetchFailureMarksFailed(t *testing.T) {
	svc, repo := newService(failingSource{err: errors.New("blocked by youtube")}, 1)
	v, err := svc.Enqueue(context.Background(), "EEEEEEEEEEE", "")
	require.NoError(t, err)
	failed := waitStatus(t, repo, v.ID, domain.StatusFailed)
	assert.Contains(t, failed.ErrorMessage, "blocked by youtube")
}

func TestNoSubtitlesMarksFailed(t *testing.T) {
	svc, repo := newService(noSubsSource{}, 1)
	v, err := svc.Enqueue(context.Background(), "FFFFFFFFFFF", "")
	require.NoError(t, err)
	failed := waitStatus(t, repo, v.ID, domain.StatusFailed)
	assert.Contains(t, failed.ErrorMessage, "no usable subtitles")
}

func TestDeleteRequiresOwner(t *testing.T) {
	svc, repo := newService(fake.SubtitleSource{}, 1)
	ctx := context.Background()
	v, err := svc.Enqueue(ctx, "GGGGGGGGGGG", "owner-hash")
	require.NoError(t, err)
	waitStatus(t, repo, v.ID, domain.StatusReady)

	assert.ErrorIs(t, svc.Delete(ctx, v.ID, "someone-else"), domain.ErrNotOwner)
	assert.ErrorIs(t, svc.Delete(ctx, v.ID, ""), domain.ErrNotOwner)
	require.NoError(t, svc.Delete(ctx, v.ID, "owner-hash"))
	_, err = repo.ByID(ctx, v.ID)
	assert.ErrorIs(t, err, domain.ErrVideoNotFound)
}
