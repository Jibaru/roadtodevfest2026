// Package repofetch downloads a public GitHub repository as a tarball and
// exposes it as an in-memory snapshot the reviewer agents can read from.
// One HTTP request per repo, no GitHub API rate limits, nothing on disk.
package repofetch

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	maxFileBytes  = 50 * 1024        // per file
	maxTotalBytes = 2 * 1024 * 1024  // whole snapshot
	maxFiles      = 400
)

// File is one text file from the repository.
type File struct {
	Path    string `json:"path"`
	Size    int    `json:"size"`
	Content string `json:"-"`
}

// Snapshot is an immutable in-memory view of a repository.
type Snapshot struct {
	Owner string
	Repo  string
	Files []File
	byPath map[string]int
}

// List returns file paths and sizes (no contents).
func (s *Snapshot) List() []File {
	out := make([]File, len(s.Files))
	for i, f := range s.Files {
		out[i] = File{Path: f.Path, Size: f.Size}
	}
	return out
}

// Read returns the content of one file.
func (s *Snapshot) Read(p string) (string, error) {
	if i, ok := s.byPath[strings.TrimPrefix(p, "/")]; ok {
		return s.Files[i].Content, nil
	}
	return "", fmt.Errorf("file not found: %s", p)
}

func (s *Snapshot) FullName() string { return s.Owner + "/" + s.Repo }

// Fetcher downloads snapshots. Implements the service's RepoFetcher port.
type Fetcher struct {
	Client *http.Client
}

// Fetch downloads owner/repo at its default branch.
func (f *Fetcher) Fetch(ctx context.Context, owner, repo string) (*Snapshot, error) {
	url := fmt.Sprintf("https://codeload.github.com/%s/%s/tar.gz/HEAD", owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading %s/%s: %w", owner, repo, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("repository %s/%s not found or not public (HTTP %d)", owner, repo, resp.StatusCode)
	}
	snap, err := fromTarball(resp.Body)
	if err != nil {
		return nil, err
	}
	snap.Owner, snap.Repo = owner, repo
	return snap, nil
}

func fromTarball(r io.Reader) (*Snapshot, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	snap := &Snapshot{byPath: map[string]int{}}
	total := 0
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg || hdr.Size > maxFileBytes || len(snap.Files) >= maxFiles {
			continue
		}
		// Strip the "repo-HEAD/" top-level directory.
		parts := strings.SplitN(hdr.Name, "/", 2)
		if len(parts) < 2 || skipPath(parts[1]) {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(tr, maxFileBytes+1))
		if err != nil || len(data) > maxFileBytes || !utf8.Valid(data) {
			continue
		}
		if total += len(data); total > maxTotalBytes {
			break
		}
		snap.byPath[parts[1]] = len(snap.Files)
		snap.Files = append(snap.Files, File{Path: parts[1], Size: len(data), Content: string(data)})
	}
	if len(snap.Files) == 0 {
		return nil, fmt.Errorf("no readable text files found in repository")
	}
	sort.Slice(snap.Files, func(i, j int) bool { return snap.Files[i].Path < snap.Files[j].Path })
	for i, f := range snap.Files {
		snap.byPath[f.Path] = i
	}
	return snap, nil
}

var skipDirs = []string{"vendor/", "node_modules/", "dist/", "build/", ".git/", "third_party/", "testdata/"}

func skipPath(p string) bool {
	for _, d := range skipDirs {
		if strings.HasPrefix(p, d) || strings.Contains(p, "/"+d) {
			return true
		}
	}
	switch strings.ToLower(path.Ext(p)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".ico", ".pdf", ".zip", ".gz", ".woff", ".woff2",
		".ttf", ".mp3", ".mp4", ".wav", ".svg", ".lock", ".sum":
		return true
	}
	return false
}
