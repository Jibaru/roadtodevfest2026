package service

import (
	"context"

	"github.com/jibaru/agentarena/internal/repofetch"
	"github.com/jibaru/agentarena/internal/review/domain"
)

// ReviewerAgent produces findings for one review dimension. Implemented
// by the ADK agents package; the service only knows this port.
// onStatus receives live progress ("reading main.go", "thinking", ...).
type ReviewerAgent interface {
	Review(ctx context.Context, reviewer domain.Reviewer, snap *repofetch.Snapshot, onStatus func(status string)) ([]domain.Finding, error)
}

// LeadReviewer writes the closing summary once all findings are in.
type LeadReviewer interface {
	Summary(ctx context.Context, repo string, findings []domain.Finding) (string, error)
}

// RepoFetcher downloads a repository snapshot.
type RepoFetcher interface {
	Fetch(ctx context.Context, owner, repo string) (*repofetch.Snapshot, error)
}

// Broadcaster fans events out to connected clients. Implemented by
// the realtime hub.
type Broadcaster interface {
	ToAudience(event Event)
	ToStage(event Event)
}
