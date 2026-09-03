package agents

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/tool"

	"github.com/jibaru/agentarena/internal/battle/domain"
)

const (
	appName   = "rapbattle"
	userID    = "show"
	modelName = "gemini-2.5-flash"
)

// battlerRuntime is one battler's agent with its own runner and a
// persistent session: the agent remembers its earlier verses across
// rounds (ADK sessions doing the comeback work for us).
type battlerRuntime struct {
	runner    *runner.Runner
	sessionID string
}

// Crew wires the whole cast: two battlers, one judge.
// It implements service.VerseWriter and service.Judge.
type Crew struct {
	battlers map[domain.Battler]*battlerRuntime
	judge    agent.Agent
	model    model.LLM

	// CrowdWords feeds the crowd-scanner tool (set from main).
	CrowdWords func(n int) []string
}

// NewCrew builds the model, the agents and their sessions.
func NewCrew(ctx context.Context, apiKey string) (*Crew, error) {
	m, err := gemini.NewModel(ctx, modelName, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("creating gemini model: %w", err)
	}

	crew := &Crew{battlers: map[domain.Battler]*battlerRuntime{}, model: m}

	instructions := map[domain.Battler]string{
		domain.BattlerBlue: blueInstruction,
		domain.BattlerRed:  redInstruction,
	}
	for battler, instruction := range instructions {
		rt, err := crew.newBattler(ctx, battlerNames[battler], instruction, nil)
		if err != nil {
			return nil, err
		}
		crew.battlers[battler] = rt
	}

	judge, err := llmagent.New(llmagent.Config{
		Name:        "the_judge",
		Model:       m,
		Description: "Ringside commentator for the rap battle.",
		Instruction: judgeInstruction,
	})
	if err != nil {
		return nil, fmt.Errorf("creating judge: %w", err)
	}
	crew.judge = judge
	return crew, nil
}

func (c *Crew) newBattler(ctx context.Context, name, instruction string, tools []tool.Tool) (*battlerRuntime, error) {
	a, err := llmagent.New(llmagent.Config{
		Name:        name,
		Model:       c.model,
		Description: "Battle rapper " + name,
		Instruction: instruction,
		Tools:       tools,
	})
	if err != nil {
		return nil, fmt.Errorf("creating agent %s: %w", name, err)
	}
	svc := session.InMemoryService()
	resp, err := svc.Create(ctx, &session.CreateRequest{AppName: appName, UserID: userID})
	if err != nil {
		return nil, fmt.Errorf("creating session for %s: %w", name, err)
	}
	r, err := runner.New(runner.Config{AppName: appName, Agent: a, SessionService: svc})
	if err != nil {
		return nil, fmt.Errorf("creating runner for %s: %w", name, err)
	}
	return &battlerRuntime{runner: r, sessionID: resp.Session.ID()}, nil
}

// WriteVerse implements service.VerseWriter: both battlers are called
// concurrently by the service, each through its own runner/session.
func (c *Crew) WriteVerse(ctx context.Context, battler domain.Battler, topic string, history []domain.RoundState) (string, error) {
	rt, ok := c.battlers[battler]
	if !ok {
		return "", domain.ErrInvalidBattler
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Round %d. Topic: %q.\n", len(history), topic)
	if last := lastOpponentVerse(battler, history); last != "" {
		fmt.Fprintf(&b, "Your opponent's previous verse was:\n%s\n", last)
	}
	b.WriteString("Drop your 8-bar verse now.")

	return c.run(ctx, rt.runner, rt.sessionID, b.String())
}

// Commentary implements service.Judge. The judge is stateless: a fresh
// session per round keeps it unbiased.
func (c *Crew) Commentary(ctx context.Context, state domain.BattleState) (string, error) {
	round := state.Rounds[len(state.Rounds)-1]
	prompt := fmt.Sprintf(
		"Round %d of %d. Topic: %q.\n\nBLUE GOPHER's verse:\n%s\n\nRED GOPHER's verse:\n%s\n\nAudience votes: BLUE GOPHER %d, RED GOPHER %d.\nGive your ringside commentary.",
		round.Number, state.TotalRounds, round.Topic,
		round.Verses[domain.BattlerBlue], round.Verses[domain.BattlerRed],
		round.VoteCounts[domain.BattlerBlue], round.VoteCounts[domain.BattlerRed],
	)

	svc := session.InMemoryService()
	resp, err := svc.Create(ctx, &session.CreateRequest{AppName: appName, UserID: userID})
	if err != nil {
		return "", err
	}
	r, err := runner.New(runner.Config{AppName: appName, Agent: c.judge, SessionService: svc})
	if err != nil {
		return "", err
	}
	return c.run(ctx, r, resp.Session.ID(), prompt)
}

// run sends one user message through a runner and collects the final text.
func (c *Crew) run(ctx context.Context, r *runner.Runner, sessionID, prompt string) (string, error) {
	msg := genai.NewContentFromText(prompt, genai.RoleUser)
	var out strings.Builder
	for event, err := range r.Run(ctx, userID, sessionID, msg,
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

func lastOpponentVerse(battler domain.Battler, history []domain.RoundState) string {
	opponent := battler.Opponent()
	for i := len(history) - 1; i >= 0; i-- {
		if v := history[i].Verses[opponent]; v != "" {
			return v
		}
	}
	return ""
}
