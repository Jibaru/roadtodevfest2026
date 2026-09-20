// Package ytdlp wraps the yt-dlp binary to fetch YouTube metadata and
// subtitle tracks. Faithful port of the original s1ng subtitles.ts:
// two-phase fetch designed to avoid YouTube's per-IP rate limit on the
// auto-translation endpoint.
package ytdlp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jibaru/s1ngo/internal/video/domain"
)

// PriorityLangs orders subtitle track preference. Asian languages first
// because romaji output is the user-facing value for them.
var PriorityLangs = []string{"ko", "ja", "es", "en"}

// Result is everything one video fetch produces.
type Result struct {
	Title            string
	DurationSec      int
	ThumbnailURL     string
	DeclaredLanguage string
	Description      string
	// Language of the chosen subtitle track ("" if none found).
	SubtitleLang string
	IsAuto       bool
	Cues         []Cue
}

// DetectLangFunc resolves the song's actual lyrics language from
// title+description (LLM agent in production; "" means unknown).
type DetectLangFunc func(ctx context.Context, title, description string) string

// Fetcher shells out to yt-dlp. Bin defaults to "yt-dlp" on PATH.
// CookiesFile, when set, is passed as --cookies to both phases — the
// standard escape hatch when YouTube bot-checks a datacenter IP.
type Fetcher struct {
	Bin         string
	CookiesFile string
	// ExtraArgs are appended to every yt-dlp invocation (e.g.
	// "--extractor-args youtube:player_client=web_safari") — the knob
	// for iterating on YouTube's datacenter challenges via env only.
	ExtraArgs []string
}

func (f *Fetcher) bin() string {
	if f.Bin != "" {
		return f.Bin
	}
	return "yt-dlp"
}

// Fetch runs the two-phase subtitle strategy:
//
//  1. Phase 1 — one yt-dlp call: metadata + manual subs only in our
//     priority languages. Manual subs never trigger the 429-prone
//     auto-translation endpoint.
//  2. Phase 2 — only if no manual track matched: fetch the auto-caption
//     in just the detected source language.
func (f *Fetcher) Fetch(ctx context.Context, youtubeID string, detectLang DetectLangFunc) (*Result, error) {
	dir, err := os.MkdirTemp("", "s1ngo-subs-"+youtubeID+"-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	out := filepath.Join(dir, "%(id)s")
	phase1 := []string{
		"--skip-download", "--write-info-json", "--write-subs",
		"--sub-langs", "ko,ja,es,en,ko.*,ja.*,es.*,en.*",
		"--sub-format", "vtt", "--no-progress", "--no-playlist",
		"-o", out, domain.WatchURL(youtubeID),
	}
	if stderr, err := f.run(ctx, phase1); err != nil {
		return nil, fmt.Errorf("yt-dlp: %s", summarize(stderr))
	}

	info := readJSON(filepath.Join(dir, youtubeID+".info.json"))
	res := &Result{
		Title:            str(info["title"], youtubeID),
		DurationSec:      num(info["duration"]),
		ThumbnailURL:     str(info["thumbnail"], "https://i.ytimg.com/vi/"+youtubeID+"/hqdefault.jpg"),
		DeclaredLanguage: str(info["language"], ""),
		Description:      str(info["description"], ""),
	}

	manualKeys := keys(info["subtitles"])

	// Detect the song's actual language: LLM agent → title script →
	// yt-dlp's declared language (uploader-tagged, often wrong).
	sourceLang := ""
	if detectLang != nil {
		sourceLang = detectLang(ctx, res.Title, res.Description)
	}
	if sourceLang == "" {
		sourceLang = DetectLangFromTitle(res.Title)
	}
	if sourceLang == "" && res.DeclaredLanguage != "" {
		base := strings.ToLower(strings.FieldsFunc(res.DeclaredLanguage, func(r rune) bool { return r == '-' || r == '.' })[0])
		if contains(PriorityLangs, base) {
			sourceLang = base
		}
	}

	files, _ := os.ReadDir(dir)
	picked := pickManualVTT(names(files), youtubeID, manualKeys, sourceLang)

	// Karaoke needs the lyrics in the language they are SUNG in — a
	// romanized translation track is useless to sing along. So when the
	// best manual track is a translation (or none exists), phase 2
	// fetches the auto-caption in the source language; the translation
	// track remains only as a last resort.
	if sourceLang != "" && (picked == nil || picked.lang != sourceLang) {
		phase2 := []string{
			"--skip-download", "--write-auto-subs",
			"--sub-langs", sourceLang, "--sub-format", "vtt",
			"--no-progress", "--no-playlist",
			"-o", out, domain.WatchURL(youtubeID),
		}
		// Phase 2 failures fall through silently: whatever was picked
		// in phase 1 (possibly nil) decides what happens next.
		if _, err := f.run(ctx, phase2); err == nil {
			files, _ := os.ReadDir(dir)
			if hit := findVTTForBaseLang(names(files), youtubeID, sourceLang); hit != nil {
				picked = &pickedVTT{file: hit.file, lang: sourceLang, isManual: isManualTrack(manualKeys, hit.fullLang)}
			}
		}
	}

	// Still stuck with a translation track? Then pick the most READABLE
	// one for the audience (es → en) instead of the source-hunting
	// ko-first order: a Spanish translation you can read beats romanized
	// Korean of a Japanese song.
	if picked != nil && sourceLang != "" && picked.lang != sourceLang {
		files, _ := os.ReadDir(dir)
		for _, lang := range []string{"es", "en"} {
			if hit := findVTTForBaseLang(names(files), youtubeID, lang); hit != nil && isManualTrack(manualKeys, hit.fullLang) {
				picked = &pickedVTT{file: hit.file, lang: lang, isManual: true}
				break
			}
		}
	}

	if picked == nil {
		return res, nil
	}

	raw, err := os.ReadFile(filepath.Join(dir, picked.file))
	if err != nil {
		return res, nil
	}
	cues := ParseVTT(string(raw))
	if len(cues) == 0 {
		return res, nil
	}
	res.SubtitleLang = picked.lang
	res.IsAuto = !picked.isManual
	res.Cues = cues
	return res, nil
}

type pickedVTT struct {
	file     string
	lang     string
	isManual bool
}

type vttHit struct {
	file     string
	fullLang string
}

func findVTTForBaseLang(files []string, youtubeID, baseLang string) *vttHit {
	exact := youtubeID + "." + baseLang + ".vtt"
	for _, f := range files {
		if f == exact {
			return &vttHit{file: f, fullLang: baseLang}
		}
	}
	for _, f := range files {
		if !strings.HasPrefix(f, youtubeID+".") || !strings.HasSuffix(f, ".vtt") {
			continue
		}
		middle := f[len(youtubeID)+1 : len(f)-4]
		if strings.FieldsFunc(middle, func(r rune) bool { return r == '-' || r == '.' })[0] == baseLang {
			return &vttHit{file: f, fullLang: middle}
		}
	}
	return nil
}

func isManualTrack(manualKeys []string, fullLang string) bool {
	base := strings.FieldsFunc(fullLang, func(r rune) bool { return r == '-' || r == '.' })[0]
	for _, k := range manualKeys {
		if k == fullLang || k == base ||
			strings.FieldsFunc(k, func(r rune) bool { return r == '-' || r == '.' })[0] == base {
			return true
		}
	}
	return false
}

// pickManualVTT prefers the detected source language, then the default
// priority order.
func pickManualVTT(files []string, youtubeID string, manualKeys []string, sourceLang string) *pickedVTT {
	var order []string
	if sourceLang != "" && contains(PriorityLangs, sourceLang) {
		order = append(order, sourceLang)
	}
	for _, l := range PriorityLangs {
		if !contains(order, l) {
			order = append(order, l)
		}
	}
	for _, baseLang := range order {
		if hit := findVTTForBaseLang(files, youtubeID, baseLang); hit != nil && isManualTrack(manualKeys, hit.fullLang) {
			return &pickedVTT{file: hit.file, lang: baseLang, isManual: true}
		}
	}
	return nil
}

// DetectLangFromTitle scores the title's Unicode script. Deterministic
// fallback when the LLM is unavailable; returns "" for Latin titles.
func DetectLangFromTitle(title string) string {
	ja, ko := 0, 0
	for _, c := range title {
		switch {
		case c >= 0x3040 && c <= 0x309f: // hiragana
			ja += 2
		case c >= 0x30a0 && c <= 0x30ff: // katakana
			ja += 2
		case c >= 0xac00 && c <= 0xd7a3: // hangul syllables
			ko += 2
		case c >= 0x1100 && c <= 0x11ff: // hangul jamo
			ko++
		case c >= 0x3130 && c <= 0x318f: // hangul compat jamo
			ko++
		case c >= 0x4e00 && c <= 0x9fff: // CJK ideographs: treat as ja in our domain
			ja++
		}
	}
	if ko >= 2 && ko >= ja {
		return "ko"
	}
	if ja >= 2 {
		return "ja"
	}
	return ""
}

func (f *Fetcher) run(ctx context.Context, args []string) (string, error) {
	// We only ever extract metadata and subtitle tracks, never video
	// formats — some player clients (web_safari) expose no formats at
	// all, and without this flag yt-dlp treats that as a hard error.
	args = append([]string{"--ignore-no-formats-error"}, args...)
	if f.CookiesFile != "" {
		args = append([]string{"--cookies", f.CookiesFile}, args...)
	}
	args = append(args, f.ExtraArgs...)
	cmd := exec.CommandContext(ctx, f.bin(), args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stderr.String(), err
}

func summarize(stderr string) string {
	var lines []string
	for _, l := range strings.Split(stderr, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(lines[i], "ERROR") {
			return truncate(lines[i], 400)
		}
	}
	if len(lines) > 0 {
		return truncate(lines[len(lines)-1], 400)
	}
	return "no stderr output"
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func readJSON(path string) map[string]any {
	raw, err := os.ReadFile(path)
	if err != nil {
		return map[string]any{}
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return map[string]any{}
	}
	return m
}

func str(v any, fallback string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return fallback
}

func num(v any) int {
	if f, ok := v.(float64); ok {
		return int(f + 0.5)
	}
	return 0
}

func keys(v any) []string {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func names(entries []os.DirEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Name()
	}
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
