package domain

import (
	"regexp"
	"strings"
)

// Pure business rules for turning subtitle cues into karaoke lines:
// each cue's duration is distributed across its words by syllable
// weight (port of the original s1ng build-lines-from-cues).

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

// CleanCues drops empty and noise-only cues. Call it before sending cue
// texts to the agents so indices stay aligned with the results.
func CleanCues(cues []SubtitleCue) []SubtitleCue {
	out := make([]SubtitleCue, 0, len(cues))
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

// BuildLyricLines creates karaoke lines from cleaned cues. displays[i]
// is the text to tokenize for cue i (the romanized form for ja/ko, the
// original otherwise); translations[i] is attached verbatim. Both
// slices may be nil to use the cue text and no translation.
func BuildLyricLines(cues []SubtitleCue, displays, translations []string) []LyricLine {
	out := make([]LyricLine, 0, len(cues))
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

		words := make([]LyricWord, 0, len(tokens))
		cursor := cue.StartMs
		for j, t := range tokens {
			endMs := cue.EndMs
			if j < len(tokens)-1 {
				share := float64(weights[j]) / float64(weightSum) * float64(totalMs)
				endMs = cursor + int(share+0.5)
			}
			words = append(words, LyricWord{Text: t, StartMs: cursor, EndMs: endMs})
			cursor = endMs
		}

		line := LyricLine{StartMs: cue.StartMs, EndMs: cue.EndMs, Words: words}
		if i < len(translations) {
			line.Translation = strings.TrimSpace(translations[i])
		}
		out = append(out, line)
	}
	return out
}
