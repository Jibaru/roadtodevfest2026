package ytdlp

import (
	"regexp"
	"strconv"
	"strings"
)

// Cue is one subtitle cue with millisecond timing.
type Cue struct {
	StartMs int
	EndMs   int
	Text    string
}

var (
	timestampRe = regexp.MustCompile(`(\d{1,2}):(\d{2}):(\d{2})\.(\d{3})\s+-->\s+(\d{1,2}):(\d{2}):(\d{2})\.(\d{3})`)
	inlineTsRe  = regexp.MustCompile(`<\d{1,2}:\d{2}:\d{2}\.\d{3}>`)
	cTagRe      = regexp.MustCompile(`</?c[^>]*>`)
	vOpenRe     = regexp.MustCompile(`<v[^>]*>`)
	anyTagRe    = regexp.MustCompile(`<[^>]+>`)
	assTagRe    = regexp.MustCompile(`\{\\[^}]+\}`)
	bracketRe   = regexp.MustCompile(`\[[^\]]+\]`)
	spaceRe     = regexp.MustCompile(`\s+`)
)

// ParseVTT extracts cues from a WebVTT document, cleaning YouTube's
// inline word timestamps and styling tags, then collapsing the
// "rolling" cues of auto-generated captions. Faithful port of the
// original's parseVtt + dedupeCues.
func ParseVTT(content string) []Cue {
	normalized := strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "\n")
	blocks := regexp.MustCompile(`\n\n+`).Split(normalized, -1)
	var cues []Cue

	for _, block := range blocks {
		var lines []string
		for _, l := range strings.Split(block, "\n") {
			if l != "" {
				lines = append(lines, l)
			}
		}
		if len(lines) < 2 {
			continue
		}

		tsIdx := -1
		var m []string
		for i, l := range lines {
			if mm := timestampRe.FindStringSubmatch(l); mm != nil {
				tsIdx, m = i, mm
				break
			}
		}
		if tsIdx == -1 {
			continue
		}

		startMs := hmsToMs(m[1], m[2], m[3], m[4])
		endMs := hmsToMs(m[5], m[6], m[7], m[8])
		if endMs <= startMs {
			continue
		}

		text := cleanCueText(strings.Join(lines[tsIdx+1:], " "))
		if text == "" {
			continue
		}
		cues = append(cues, Cue{StartMs: startMs, EndMs: endMs, Text: text})
	}
	return dedupeCues(cues)
}

func cleanCueText(raw string) string {
	s := inlineTsRe.ReplaceAllString(raw, "")
	s = cTagRe.ReplaceAllString(s, "")
	s = vOpenRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "</v>", "")
	s = anyTagRe.ReplaceAllString(s, "")
	s = assTagRe.ReplaceAllString(s, "")
	s = bracketRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "♪", "")
	for old, nw := range map[string]string{
		"&amp;": "&", "&lt;": "<", "&gt;": ">", "&quot;": `"`, "&#39;": "'",
	} {
		s = strings.ReplaceAll(s, old, nw)
	}
	return strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
}

// dedupeCues collapses runs where one cue's text is a strict prefix of
// the next (YouTube auto-subs emit "rolling" cues), keeping the longest.
func dedupeCues(cues []Cue) []Cue {
	if len(cues) == 0 {
		return cues
	}
	var out []Cue
	for _, cue := range cues {
		if len(out) > 0 {
			last := &out[len(out)-1]
			if cue.Text == last.Text {
				if cue.EndMs > last.EndMs {
					last.EndMs = cue.EndMs
				}
				continue
			}
			if strings.HasPrefix(cue.Text, last.Text) && cue.StartMs <= last.EndMs+200 {
				out[len(out)-1] = Cue{StartMs: last.StartMs, EndMs: cue.EndMs, Text: cue.Text}
				continue
			}
		}
		out = append(out, cue)
	}
	return out
}

func hmsToMs(hh, mm, ss, fff string) int {
	h, _ := strconv.Atoi(hh)
	m, _ := strconv.Atoi(mm)
	s, _ := strconv.Atoi(ss)
	f, _ := strconv.Atoi(fff)
	return h*3_600_000 + m*60_000 + s*1_000 + f
}
