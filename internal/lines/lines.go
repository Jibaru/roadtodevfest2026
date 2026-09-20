// Package lines turns subtitle cues into karaoke lines with word-level
// timing. Port of the original build-lines-from-cues.ts: each cue's
// duration is distributed across its tokens by syllable weight.
package lines

import (
	"regexp"
	"strings"

	"github.com/jibaru/s1ngo/internal/video/domain"
	"github.com/jibaru/s1ngo/internal/ytdlp"
)

var (
	vowelRe     = regexp.MustCompile(`[aeiouAEIOUáéíóúÁÉÍÓÚâêîôûÂÊÎÔÛ]`)
	noiseOnlyRe = regexp.MustCompile(`^[\[(♪].*[\])♪]$|^\.\.\.$`)
)

func syllableWeight(token string) int {
	n := len(vowelRe.FindAllString(token, -1))
	if n < 1 {
		return 1
	}
	return n
}

// CleanCues drops empty/noise-only cues. Call before feeding cue texts
// to the romanizer/translator agents so indices stay aligned.
func CleanCues(cues []ytdlp.Cue) []ytdlp.Cue {
	out := make([]ytdlp.Cue, 0, len(cues))
	for _, c := range cues {
		text := strings.TrimSpace(c.Text)
		if text == "" || noiseOnlyRe.MatchString(text) {
			continue
		}
		c.Text = text
		out = append(out, c)
	}
	return out
}

// Build creates karaoke lines from cleaned cues. displays[i] is the text
// to tokenize for cue i (the romanized form for ja/ko, the original
// otherwise); translations[i] is attached verbatim (may be empty).
// Both slices may be nil to use the cue text and no translation.
func Build(cues []ytdlp.Cue, displays, translations []string) []domain.LyricLine {
	out := make([]domain.LyricLine, 0, len(cues))
	for i, cue := range cues {
		display := cue.Text
		if i < len(displays) && strings.TrimSpace(displays[i]) != "" {
			display = strings.TrimSpace(displays[i])
		}
		tokens := strings.Fields(display)
		if len(tokens) == 0 {
			continue
		}

		totalMs := cue.EndMs - cue.StartMs
		if totalMs < 1 {
			totalMs = 1
		}
		weights := make([]int, len(tokens))
		weightSum := 0
		for j, t := range tokens {
			weights[j] = syllableWeight(t)
			weightSum += weights[j]
		}

		words := make([]domain.LyricWord, 0, len(tokens))
		cursor := cue.StartMs
		for j, t := range tokens {
			endMs := cue.EndMs
			if j < len(tokens)-1 {
				share := float64(weights[j]) / float64(weightSum) * float64(totalMs)
				endMs = cursor + int(share+0.5)
			}
			words = append(words, domain.LyricWord{Text: t, StartMs: cursor, EndMs: endMs})
			cursor = endMs
		}

		line := domain.LyricLine{StartMs: cue.StartMs, EndMs: cue.EndMs, Words: words}
		if i < len(translations) {
			line.Translation = strings.TrimSpace(translations[i])
		}
		out = append(out, line)
	}
	return out
}

// NeedsRomanization mirrors the original: only ja/ko scripts need it.
func NeedsRomanization(language string) bool {
	switch language {
	case "ja", "japanese", "ko", "korean":
		return true
	}
	return false
}
