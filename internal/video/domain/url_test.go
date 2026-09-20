package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jibaru/s1ngo/internal/video/domain"
)

func TestParseYouTubeID(t *testing.T) {
	for input, want := range map[string]string{
		"dQw4w9WgXcQ": "dQw4w9WgXcQ",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ":       "dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ?t=42":                 "dQw4w9WgXcQ",
		"https://m.youtube.com/watch?v=dQw4w9WgXcQ&list=xx": "dQw4w9WgXcQ",
		"https://music.youtube.com/watch?v=dQw4w9WgXcQ":     "dQw4w9WgXcQ",
		"https://www.youtube.com/shorts/dQw4w9WgXcQ":        "dQw4w9WgXcQ",
		"https://www.youtube.com/embed/dQw4w9WgXcQ":         "dQw4w9WgXcQ",
		"https://www.youtube.com/live/dQw4w9WgXcQ":          "dQw4w9WgXcQ",
	} {
		got, err := domain.ParseYouTubeID(input)
		require.NoError(t, err, input)
		assert.Equal(t, want, got, input)
	}
	for _, bad := range []string{"", "short", "https://vimeo.com/12345", "https://youtube.com/watch?v=tooshort"} {
		_, err := domain.ParseYouTubeID(bad)
		assert.ErrorIs(t, err, domain.ErrInvalidYouTubeURL, bad)
	}
}
