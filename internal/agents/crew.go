package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"

	"google.golang.org/genai"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"

	"github.com/jibaru/agentarena/internal/repofetch"
	"github.com/jibaru/agentarena/internal/review/domain"
)

const (
	appName      = "agentarena"
	userID       = "show"
	modelName    = "gemini-2.5-flash"
	maxFileReads = 12 // hard budget per reviewer per round
	maxListLines = 250
)

// Crew is the reviewing cast: three specialist reviewers plus the lead.
// It implements service.ReviewerAgent and service.LeadReviewer.
type Crew struct {
	model model.LLM
}

// NewCrew builds the shared Gemini model.
func NewCrew(ctx context.Context, apiKey string) (*Crew, error) {
	m, err := gemini.NewModel(ctx, modelName, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("creating gemini model: %w", err)
	}
	return &Crew{model: m}, nil
}

// ReadArgs / ReadResult define the read_file tool contract: the typed
// structs ARE the schema the model sees (inferred by reflection).
type ReadArgs struct {
	Path string `json:"path"` // file path exactly as it appears in the listing
}

type ReadResult struct {
	Content string `json:"content"`
}

// Review implements service.ReviewerAgent: a fresh agent per round,
// armed with a read_file tool bound to this repo snapshot. Every tool
// call surfaces through onStatus so the audience watches the agent
// choose which files to open.
func (c *Crew) Review(ctx context.Context, reviewer domain.Reviewer, snap *repofetch.Snapshot, onStatus func(string)) ([]domain.Finding, error) {
	instruction, ok := reviewerInstructions[reviewer]
	if !ok {
		return nil, domain.ErrInvalidReviewer
	}

	var reads atomic.Int32
	readFile, err := functiontool.New(functiontool.Config{
		Name:        "read_file",
		Description: "Read one file from the repository under review. Use the exact path from the listing.",
	}, func(_ agent.Context, args ReadArgs) (ReadResult, error) {
		if reads.Add(1) > maxFileReads {
			return ReadResult{}, fmt.Errorf("read budget exhausted — output your findings now")
		}
		onStatus("reading " + args.Path)
		content, err := snap.Read(args.Path)
		if err != nil {
			return ReadResult{}, err
		}
		return ReadResult{Content: content}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("creating read_file tool: %w", err)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        reviewerNames[reviewer],
		Model:       c.model,
		Description: "Specialist code reviewer: " + string(reviewer),
		Instruction: instruction,
		Tools:       []tool.Tool{readFile},
	})
	if err != nil {
		return nil, fmt.Errorf("creating reviewer %s: %w", reviewer, err)
	}

	prompt := fmt.Sprintf("Repository: %s\n\nFile listing (path · bytes):\n%s\nRead what matters for your specialty, then output your findings JSON.",
		snap.FullName(), formatListing(snap))

	onStatus("scanning the file tree")
	out, err := c.run(ctx, a, prompt)
	if err != nil {
		return nil, err
	}
	onStatus("writing up findings")
	return parseFindings(out)
}

// Summary implements service.LeadReviewer.
func (c *Crew) Summary(ctx context.Context, repo string, findings []domain.Finding) (string, error) {
	lead, err := llmagent.New(llmagent.Config{
		Name:        "lead_reviewer",
		Model:       c.model,
		Description: "Closes the review with a verdict.",
		Instruction: leadInstruction,
	})
	if err != nil {
		return "", err
	}
	data, _ := json.Marshal(findings)
	prompt := fmt.Sprintf("Repository: %s\nFindings:\n%s\nGive your closing summary.", repo, data)
	return c.run(ctx, lead, prompt)
}

// run executes one agent with a fresh session and collects the final text.
func (c *Crew) run(ctx context.Context, a agent.Agent, prompt string) (string, error) {
	svc := session.InMemoryService()
	resp, err := svc.Create(ctx, &session.CreateRequest{AppName: appName, UserID: userID})
	if err != nil {
		return "", err
	}
	r, err := runner.New(runner.Config{AppName: appName, Agent: a, SessionService: svc})
	if err != nil {
		return "", err
	}

	msg := genai.NewContentFromText(prompt, genai.RoleUser)
	var out strings.Builder
	for event, err := range r.Run(ctx, userID, resp.Session.ID(), msg,
		agent.RunConfig{StreamingMode: agent.StreamingModeNone}) {
		if err != nil {
			return "", err
		}
		if event.LLMResponse.Content == nil || !event.IsFinalResponse() {
			continue
		}
		for _, p := range event.LLMResponse.Content.Parts {
			out.WriteString(p.Text)
		}
	}
	return strings.TrimSpace(out.String()), nil
}

func formatListing(snap *repofetch.Snapshot) string {
	var b strings.Builder
	for i, f := range snap.List() {
		if i >= maxListLines {
			fmt.Fprintf(&b, "… and %d more files\n", len(snap.Files)-maxListLines)
			break
		}
		fmt.Fprintf(&b, "%s · %d\n", f.Path, f.Size)
	}
	return b.String()
}

// parseFindings tolerantly extracts the JSON array from model output.
func parseFindings(out string) ([]domain.Finding, error) {
	start := strings.Index(out, "[")
	end := strings.LastIndex(out, "]")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("no findings JSON in output: %.120s", out)
	}
	var raw []struct {
		Severity   string `json:"severity"`
		Title      string `json:"title"`
		File       string `json:"file"`
		Detail     string `json:"detail"`
		Suggestion string `json:"suggestion"`
	}
	if err := json.Unmarshal([]byte(out[start:end+1]), &raw); err != nil {
		return nil, fmt.Errorf("parsing findings JSON: %w", err)
	}
	findings := make([]domain.Finding, 0, len(raw))
	for _, f := range raw {
		findings = append(findings, domain.Finding{
			Severity:   f.Severity,
			Title:      f.Title,
			File:       f.File,
			Detail:     f.Detail,
			Suggestion: f.Suggestion,
		})
	}
	return findings, nil
}
