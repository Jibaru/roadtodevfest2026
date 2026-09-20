package ytdlp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleVTT = `WEBVTT
Kind: captions
Language: en

00:00:01.000 --> 00:00:03.500
<c>goodbye</c> <00:00:02.000>moon ♪

00:00:03.500 --> 00:00:05.000
hello world again

00:00:05.000 --> 00:00:06.000
hello world again

00:00:06.000 --> 00:00:08.000
hello world again and more

00:00:08.500 --> 00:00:09.000
[Music]
`

func TestParseVTT(t *testing.T) {
	cues := ParseVTT(sampleVTT)
	require.Len(t, cues, 2, "rolling cues collapsed, noise dropped")

	assert.Equal(t, Cue{StartMs: 1000, EndMs: 3500, Text: "goodbye moon"}, cues[0])
	// The three "hello world again..." cues are a rolling run: identical
	// text merges, the prefix-extension replaces keeping the first start.
	assert.Equal(t, 3500, cues[1].StartMs)
	assert.Equal(t, 8000, cues[1].EndMs)
	assert.Equal(t, "hello world again and more", cues[1].Text)
}

func TestDetectLangFromTitle(t *testing.T) {
	assert.Equal(t, "ja", DetectLangFromTitle("YOASOBI「アイドル」Official Music Video"))
	assert.Equal(t, "ko", DetectLangFromTitle("아이유 (IU) - 좋은 날"))
	assert.Equal(t, "", DetectLangFromTitle("Never Gonna Give You Up"))
}
