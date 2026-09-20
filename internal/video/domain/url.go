package domain

import (
	"net/url"
	"regexp"
	"strings"
)

var idRe = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// ParseYouTubeID accepts a bare 11-char id, youtu.be links, and
// youtube.com watch/shorts/embed/live URLs. Faithful port of the
// original's url.ts.
func ParseYouTubeID(input string) (string, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return "", ErrInvalidYouTubeURL
	}
	if idRe.MatchString(raw) {
		return raw, nil
	}

	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", ErrInvalidYouTubeURL
	}
	host := strings.TrimPrefix(u.Hostname(), "www.")

	if host == "youtu.be" {
		id := strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 2)[0]
		if idRe.MatchString(id) {
			return id, nil
		}
	}

	if host == "youtube.com" || host == "m.youtube.com" || host == "music.youtube.com" {
		if v := u.Query().Get("v"); idRe.MatchString(v) {
			return v, nil
		}
		segs := strings.FieldsFunc(u.Path, func(r rune) bool { return r == '/' })
		if len(segs) >= 2 && (segs[0] == "shorts" || segs[0] == "embed" || segs[0] == "live") {
			if idRe.MatchString(segs[1]) {
				return segs[1], nil
			}
		}
	}
	return "", ErrInvalidYouTubeURL
}

// WatchURL returns the canonical watch URL for an id.
func WatchURL(youtubeID string) string {
	return "https://www.youtube.com/watch?v=" + youtubeID
}
