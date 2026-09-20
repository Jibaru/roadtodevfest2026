package lines

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jibaru/s1ngo/internal/ytdlp"
)

func TestBuildDistributesTimeBySyllables(t *testing.T) {
	cues := []ytdlp.Cue{{StartMs: 0, EndMs: 1000, Text: "hi lalala"}}
	built := Build(cues, nil, []string{"hola lalala"})
	require.Len(t, built, 1)
	line := built[0]
	require.Len(t, line.Words, 2)

	// "hi" = 1 vowel, "lalala" = 3 vowels → 250ms / 750ms split.
	assert.Equal(t, 0, line.Words[0].StartMs)
	assert.Equal(t, 250, line.Words[0].EndMs)
	assert.Equal(t, 250, line.Words[1].StartMs)
	assert.Equal(t, 1000, line.Words[1].EndMs, "last word always ends at cue end")
	assert.Equal(t, "hola lalala", line.Translation)
}

func TestBuildUsesDisplayOverride(t *testing.T) {
	cues := []ytdlp.Cue{{StartMs: 0, EndMs: 900, Text: "アイドル"}}
	built := Build(cues, []string{"a i do ru"}, nil)
	require.Len(t, built, 1)
	assert.Len(t, built[0].Words, 4, "romanized display is what gets tokenized")
}

func TestCleanCuesDropsNoise(t *testing.T) {
	cues := CleanCues([]ytdlp.Cue{
		{StartMs: 0, EndMs: 1, Text: "  real line  "},
		{StartMs: 1, EndMs: 2, Text: "(♪)"},
		{StartMs: 2, EndMs: 3, Text: "..."},
		{StartMs: 3, EndMs: 4, Text: ""},
	})
	require.Len(t, cues, 1)
	assert.Equal(t, "real line", cues[0].Text)
}

func TestNeedsRomanization(t *testing.T) {
	assert.True(t, NeedsRomanization("ja"))
	assert.True(t, NeedsRomanization("ko"))
	assert.False(t, NeedsRomanization("es"))
	assert.False(t, NeedsRomanization("en"))
}
