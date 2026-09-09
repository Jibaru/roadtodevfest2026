// Package fake provides no-API rehearsal doubles for the reviewing crew
// and the repo fetcher. Run the whole show offline with FAKE_AGENTS=1.
package fake

import (
	"context"
	"fmt"
	"time"

	"github.com/jibaru/agentarena/internal/repofetch"
	"github.com/jibaru/agentarena/internal/review/domain"
)

// Crew fakes the three reviewers and the lead with canned findings.
type Crew struct {
	// Delay simulates model latency so the "reviewing" phase is visible.
	Delay time.Duration
}

var cannedFindings = map[domain.Reviewer][]domain.Finding{
	domain.ReviewerBugs: {
		{Severity: "high", Title: "Unchecked error on file close", File: "main.go",
			Detail: "The error returned by Close() is discarded, which can silently lose buffered writes.",
			Suggestion: "Handle or at least log the Close() error on the write path."},
		{Severity: "medium", Title: "Map read without lock", File: "internal/state.go",
			Detail: "The sessions map is read from multiple goroutines without synchronization.",
			Suggestion: "Guard the map with a sync.RWMutex or use sync.Map."},
	},
	domain.ReviewerSecurity: {
		{Severity: "high", Title: "Secret committed in config", File: "config/dev.yaml",
			Detail: "A credential-looking string is hardcoded in a tracked file.",
			Suggestion: "Move secrets to environment variables and rotate the exposed one."},
		{Severity: "low", Title: "Missing request size limit", File: "server/handler.go",
			Detail: "Request bodies are read without a cap, enabling memory exhaustion.",
			Suggestion: "Wrap the body with http.MaxBytesReader."},
	},
	domain.ReviewerSimplify: {
		{Severity: "medium", Title: "One function doing five jobs", File: "internal/process.go",
			Detail: "process() parses, validates, transforms, saves and notifies — 120 lines of mixed concerns.",
			Suggestion: "Split it along those five verbs; each piece becomes independently testable."},
		{Severity: "low", Title: "Dead configuration flags", File: "config/config.go",
			Detail: "Three flags are parsed but never read anywhere in the codebase.",
			Suggestion: "Delete them; less surface, less confusion."},
	},
}

func (c *Crew) Review(ctx context.Context, reviewer domain.Reviewer, snap *repofetch.Snapshot, onStatus func(string)) ([]domain.Finding, error) {
	for _, f := range snap.List() {
		onStatus("reading " + f.Path)
		select {
		case <-time.After(c.Delay / 3):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return cannedFindings[reviewer], nil
}

func (c *Crew) Summary(_ context.Context, repo string, findings []domain.Finding) (string, error) {
	return fmt.Sprintf("Solid bones in %s, but the crew surfaced %d things worth fixing — start with the high-severity ones and this repo is in great shape.",
		repo, len(findings)), nil
}

// Fetcher returns a tiny embedded snapshot so rehearsals never hit the network.
type Fetcher struct{}

func (Fetcher) Fetch(_ context.Context, owner, repo string) (*repofetch.Snapshot, error) {
	files := []repofetch.File{
		{Path: "main.go", Size: 420, Content: "package main\n\nfunc main() {}\n"},
		{Path: "internal/state.go", Size: 380, Content: "package internal\n\nvar sessions = map[string]int{}\n"},
		{Path: "config/dev.yaml", Size: 96, Content: "api_key: fake-not-a-real-secret\n"},
	}
	snap := &repofetch.Snapshot{Owner: owner, Repo: repo, Files: files}
	return snap, nil
}
