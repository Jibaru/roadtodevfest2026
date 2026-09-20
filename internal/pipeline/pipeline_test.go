package pipeline_test

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jibaru/s1ngo/internal/agents/fake"
	"github.com/jibaru/s1ngo/internal/pipeline"
	"github.com/jibaru/s1ngo/internal/video/domain"
	"github.com/jibaru/s1ngo/internal/video/infra/persistence/memory"
	"github.com/jibaru/s1ngo/internal/ytdlp"
)

type failingFetcher struct{ err error }

func (f failingFetcher) Fetch(context.Context, string, ytdlp.DetectLangFunc) (*ytdlp.Result, error) {
	return nil, f.err
}

type noSubsFetcher struct{}

func (noSubsFetcher) Fetch(_ context.Context, id string, _ ytdlp.DetectLangFunc) (*ytdlp.Result, error) {
	return &ytdlp.Result{Title: "No Subs " + id, ThumbnailURL: "t", DurationSec: 10}, nil
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

func newPipe(fetcher pipeline.Fetcher, workers int) (*pipeline.Service, domain.VideoRepository) {
	repo := memory.NewVideoRepository()
	pipe := pipeline.New(repo, fetcher, &fake.Crew{Delay: 10 * time.Millisecond}, &castRecorder{}, workers, slog.New(slog.DiscardHandler))
	return pipe, repo
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

func TestPipelineProcessesVideo(t *testing.T) {
	pipe, repo := newPipe(fake.Fetcher{}, 2)
	ctx := context.Background()

	v, err := pipe.Enqueue(ctx, "https://youtu.be/dQw4w9WgXcQ", "fp-hash")
	require.NoError(t, err)
	assert.Equal(t, domain.StatusProcessing, v.Status)

	ready := waitStatus(t, repo, v.ID, domain.StatusReady)
	assert.Equal(t, "en", ready.Language)
	require.Len(t, ready.Lyrics, 3, "fake song has 3 lines")
	assert.NotEmpty(t, ready.Lyrics[0].Words)
	assert.Contains(t, ready.Lyrics[0].Translation, "[es]", "translator agent ran")
	assert.Contains(t, ready.Title, "Fake Song", "metadata updated after fetch")
}

func TestPipelineParallelism(t *testing.T) {
	pipe, repo := newPipe(fake.Fetcher{}, 4)
	ctx := context.Background()

	ids := []string{"AAAAAAAAAAA", "BBBBBBBBBBB", "CCCCCCCCCCC", "DDDDDDDDDDD"}
	var videos []*domain.Video
	for _, y := range ids {
		v, err := pipe.Enqueue(ctx, y, "")
		require.NoError(t, err)
		videos = append(videos, v)
	}
	for _, v := range videos {
		waitStatus(t, repo, v.ID, domain.StatusReady)
	}
}

func TestEnqueueDeduplicates(t *testing.T) {
	pipe, repo := newPipe(fake.Fetcher{}, 1)
	ctx := context.Background()

	v1, err := pipe.Enqueue(ctx, "dQw4w9WgXcQ", "")
	require.NoError(t, err)
	waitStatus(t, repo, v1.ID, domain.StatusReady)

	// Same video again → returns the existing record, no new job.
	v2, err := pipe.Enqueue(ctx, "https://youtu.be/dQw4w9WgXcQ", "")
	require.NoError(t, err)
	assert.Equal(t, v1.ID, v2.ID)

	_, err = pipe.Enqueue(ctx, "not a url", "")
	assert.ErrorIs(t, err, domain.ErrInvalidYouTubeURL)
}

func TestFetchFailureMarksFailed(t *testing.T) {
	pipe, repo := newPipe(failingFetcher{err: errors.New("blocked by youtube")}, 1)
	v, err := pipe.Enqueue(context.Background(), "EEEEEEEEEEE", "")
	require.NoError(t, err)
	failed := waitStatus(t, repo, v.ID, domain.StatusFailed)
	assert.Contains(t, failed.ErrorMessage, "blocked by youtube")
}

func TestNoSubtitlesMarksFailed(t *testing.T) {
	pipe, repo := newPipe(noSubsFetcher{}, 1)
	v, err := pipe.Enqueue(context.Background(), "FFFFFFFFFFF", "")
	require.NoError(t, err)
	failed := waitStatus(t, repo, v.ID, domain.StatusFailed)
	assert.Contains(t, failed.ErrorMessage, "no usable subtitles")
}

func TestDeleteRequiresOwner(t *testing.T) {
	pipe, repo := newPipe(fake.Fetcher{}, 1)
	ctx := context.Background()
	v, err := pipe.Enqueue(ctx, "GGGGGGGGGGG", "owner-hash")
	require.NoError(t, err)
	waitStatus(t, repo, v.ID, domain.StatusReady)

	assert.ErrorIs(t, pipe.Delete(ctx, v.ID, "someone-else"), domain.ErrNotOwner)
	assert.ErrorIs(t, pipe.Delete(ctx, v.ID, ""), domain.ErrNotOwner)
	require.NoError(t, pipe.Delete(ctx, v.ID, "owner-hash"))
	_, err = repo.ByID(ctx, v.ID)
	assert.ErrorIs(t, err, domain.ErrVideoNotFound)
}
