// Package agents implements the service.LyricsAgents port with three
// ADK agents — a language detective, a romanizer and a translator —
// backed by an OpenAI model (GPT-6 Luna by default).
package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/openaimodel"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
)

const (
	appName = "s1ngo"
	userID  = "processing"
	// Lyrics batches are chunked small: besides bounding output size,
	// short fragments avoid the model balking at mapping a whole
	// well-known song in one go.
	chunkSize = 12
)

// OpenAIAgents implements service.LyricsAgents.
type OpenAIAgents struct {
	model model.LLM
}

// NewOpenAIAgents builds the shared model once; each call then runs a
// fresh single-turn agent on top of it.
func NewOpenAIAgents(ctx context.Context, modelName, apiKey string) (*OpenAIAgents, error) {
	m, err := openaimodel.NewModel(ctx, modelName, &openaimodel.ClientConfig{APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("creating openai model %q: %w", modelName, err)
	}
	return &OpenAIAgents{model: m}, nil
}

// DetectLanguage returns ja/ko/es/en, or "" when unknown/unsupported —
// letting the caller fall back to deterministic title-script detection.
func (o *OpenAIAgents) DetectLanguage(ctx context.Context, title, description string) (string, error) {
	desc := description
	if len(desc) > 600 {
		desc = desc[:600]
	}
	out, err := o.runAgent(ctx, "language_detective", detectInstruction,
		fmt.Sprintf("Title: %s\nDescription: %s", title, desc))
	if err != nil {
		return "", err
	}
	lang := strings.ToLower(strings.TrimSpace(out))
	switch lang {
	case "ja", "ko", "es", "en":
		return lang, nil
	}
	return "", nil
}

// Romanize converts ja/ko lyric lines to Latin script, batch by batch.
func (o *OpenAIAgents) Romanize(ctx context.Context, language string, texts []string) ([]string, error) {
	return o.mapLines(ctx, "romanizer", romanizeInstruction,
		"Language: "+language, texts)
}

// Translate renders each lyric line in the target language: Spanish for
// most songs, English when the song is already in Spanish.
func (o *OpenAIAgents) Translate(ctx context.Context, sourceLang string, texts []string) ([]string, error) {
	target := "Latin American Spanish"
	if sourceLang == "es" {
		target = "English"
	}
	return o.mapLines(ctx, "translator", fmt.Sprintf(translateInstruction, target),
		"Source language: "+sourceLang, texts)
}

// mapLines sends texts through an agent in chunks, expecting a JSON
// array of the same length back for each chunk.
func (o *OpenAIAgents) mapLines(ctx context.Context, name, instruction, header string, texts []string) ([]string, error) {
	out := make([]string, 0, len(texts))
	for start := 0; start < len(texts); start += chunkSize {
		end := min(start+chunkSize, len(texts))
		chunk := texts[start:end]

		payload, _ := json.Marshal(chunk)
		prompt := fmt.Sprintf("%s\nThe array has EXACTLY %d elements; return EXACTLY %d.\nLines:\n%s",
			header, len(chunk), len(chunk), payload)

		// LLMs occasionally merge or drop a line; one retry recovers
		// most transient contract violations.
		var mapped []string
		var lastErr error
		for attempt := 0; attempt < 2; attempt++ {
			reply, err := o.runAgent(ctx, name, instruction, prompt)
			if err != nil {
				lastErr = err
				continue
			}
			if mapped, err = parseStringArray(reply, len(chunk)); err == nil {
				break
			}
			lastErr = err
		}
		if mapped == nil {
			return nil, fmt.Errorf("%s: %w", name, lastErr)
		}
		out = append(out, mapped...)
	}
	return out, nil
}

// runAgent executes one single-turn agent with a fresh session.
func (o *OpenAIAgents) runAgent(ctx context.Context, name, instruction, prompt string) (string, error) {
	a, err := llmagent.New(llmagent.Config{
		Name:        name,
		Model:       o.model,
		Description: "s1n.go " + name,
		Instruction: instruction,
	})
	if err != nil {
		return "", err
	}
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
	var sb strings.Builder
	for event, err := range r.Run(ctx, userID, resp.Session.ID(), msg,
		agent.RunConfig{StreamingMode: agent.StreamingModeNone}) {
		if err != nil {
			return "", err
		}
		if event.LLMResponse.Content == nil || !event.IsFinalResponse() {
			continue
		}
		for _, p := range event.LLMResponse.Content.Parts {
			sb.WriteString(p.Text)
		}
	}
	return strings.TrimSpace(sb.String()), nil
}

// parseStringArray tolerantly extracts a JSON string array of exactly
// n elements from model output.
func parseStringArray(out string, n int) ([]string, error) {
	start := strings.Index(out, "[")
	end := strings.LastIndex(out, "]")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("no JSON array in output: %.120s", out)
	}
	var arr []string
	if err := json.Unmarshal([]byte(out[start:end+1]), &arr); err != nil {
		return nil, fmt.Errorf("parsing array: %w", err)
	}
	if len(arr) != n {
		return nil, fmt.Errorf("expected %d lines, got %d", n, len(arr))
	}
	return arr, nil
}
