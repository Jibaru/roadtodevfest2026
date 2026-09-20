package domain

import "errors"

// Domain errors. Infrastructure implementations must return these,
// never their own driver-specific errors.
var (
	ErrVideoNotFound     = errors.New("video not found")
	ErrInvalidYouTubeURL = errors.New("not a recognizable YouTube URL or video id")
	ErrAlreadyQueued     = errors.New("this video is already being processed")
	ErrNotOwner          = errors.New("only the person who added a video can delete it")
	ErrNoSubtitles       = errors.New("this video has no usable subtitles (v1 is subtitles-only)")
)
