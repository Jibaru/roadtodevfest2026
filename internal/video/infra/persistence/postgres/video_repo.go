package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jibaru/s1ngo/internal/video/domain"
)

const schema = `
CREATE TABLE IF NOT EXISTS videos (
  id TEXT PRIMARY KEY,
  youtube_id TEXT NOT NULL UNIQUE,
  title TEXT NOT NULL,
  thumbnail_url TEXT NOT NULL DEFAULT '',
  duration_sec INT NOT NULL DEFAULT 0,
  language TEXT,
  status TEXT NOT NULL,
  error_message TEXT,
  lyrics JSONB,
  owner_fingerprint_hash TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);`

// VideoRepository persists videos in Postgres via pgx. Same interface
// as the memory repo.
type VideoRepository struct {
	pool *pgxpool.Pool
}

// NewVideoRepository connects and applies the schema.
func NewVideoRepository(ctx context.Context, databaseURL string) (*VideoRepository, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connecting to postgres: %w", err)
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		return nil, fmt.Errorf("applying schema: %w", err)
	}
	return &VideoRepository{pool: pool}, nil
}

func (r *VideoRepository) Create(ctx context.Context, v *domain.Video) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO videos (id, youtube_id, title, thumbnail_url, duration_sec, status, owner_fingerprint_hash, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		v.ID, v.YouTubeID, v.Title, v.ThumbnailURL, v.DurationSec, v.Status, nullable(v.OwnerFingerprintHash), v.CreatedAt)
	return err
}

const cols = `id, youtube_id, title, thumbnail_url, duration_sec,
	COALESCE(language,''), status, COALESCE(error_message,''), lyrics,
	COALESCE(owner_fingerprint_hash,''), created_at`

func (r *VideoRepository) ByID(ctx context.Context, id string) (*domain.Video, error) {
	return r.one(ctx, `SELECT `+cols+` FROM videos WHERE id=$1`, id)
}

func (r *VideoRepository) ByYouTubeID(ctx context.Context, youtubeID string) (*domain.Video, error) {
	return r.one(ctx, `SELECT `+cols+` FROM videos WHERE youtube_id=$1`, youtubeID)
}

func (r *VideoRepository) one(ctx context.Context, q string, arg any) (*domain.Video, error) {
	v, err := scan(r.pool.QueryRow(ctx, q, arg))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrVideoNotFound
	}
	return v, err
}

func (r *VideoRepository) ListRecent(ctx context.Context, limit int) ([]*domain.Video, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `SELECT `+cols+` FROM videos ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Video
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *VideoRepository) UpdateMeta(ctx context.Context, id, title, thumbnailURL string, durationSec int) error {
	return r.exec(ctx, `UPDATE videos SET title=$2, thumbnail_url=$3, duration_sec=$4 WHERE id=$1`,
		id, title, thumbnailURL, durationSec)
}

func (r *VideoRepository) MarkProcessing(ctx context.Context, id string) error {
	return r.exec(ctx, `UPDATE videos SET status='processing', error_message=NULL WHERE id=$1`, id)
}

func (r *VideoRepository) MarkReady(ctx context.Context, id string, lyrics []domain.LyricLine, language string) error {
	data, err := json.Marshal(lyrics)
	if err != nil {
		return err
	}
	return r.exec(ctx, `UPDATE videos SET status='ready', lyrics=$2, language=$3, error_message=NULL WHERE id=$1`,
		id, data, language)
}

func (r *VideoRepository) MarkFailed(ctx context.Context, id string, message string) error {
	return r.exec(ctx, `UPDATE videos SET status='failed', error_message=$2 WHERE id=$1`, id, message)
}

func (r *VideoRepository) Delete(ctx context.Context, id string) error {
	return r.exec(ctx, `DELETE FROM videos WHERE id=$1`, id)
}

func (r *VideoRepository) exec(ctx context.Context, q string, args ...any) error {
	tag, err := r.pool.Exec(ctx, q, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrVideoNotFound
	}
	return nil
}

type rowScanner interface{ Scan(dest ...any) error }

func scan(row rowScanner) (*domain.Video, error) {
	var v domain.Video
	var lyrics []byte
	if err := row.Scan(&v.ID, &v.YouTubeID, &v.Title, &v.ThumbnailURL, &v.DurationSec,
		&v.Language, &v.Status, &v.ErrorMessage, &lyrics, &v.OwnerFingerprintHash, &v.CreatedAt); err != nil {
		return nil, err
	}
	if len(lyrics) > 0 {
		if err := json.Unmarshal(lyrics, &v.Lyrics); err != nil {
			return nil, err
		}
	}
	return &v, nil
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
