// Package agents is the ADK-powered crew of s1n.go: a language
// detective, a romanizer and a translator. In the original s1ng these
// were npm libraries (kuroshiro, hangul-romanization) and a GPT call —
// here each one is a Gemini agent, because Go has no good romanization
// libraries and that's exactly what LLMs are great at.
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
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
)

const (
	appName   = "s1ngo"
	userID    = "pipeline"
	modelName = "gemini-2.5-flash"
	// Lyrics batches are chunked so one huge song can't blow the
	// output budget of a single call.
	chunkSize = 60
)

const detectInstruction = `You identify the language that a song's LYRICS are sung in,
from its YouTube title and description.

Rules:
- Return the language of the LYRICS, not the title. "BTS - Dynamite (Korean cover)" → ko.
- For covers and dubs, return the language OF THIS RECORDING:
  "Momoland - Baam Baam Japanese Version" → ja, even though the original is Korean.
- Use the description for clues (cover language, lyrics excerpts, original artist).
- Reply with EXACTLY one token: ja, ko, es, en, or other.
- If unsure, reply other.`

const romanizeInstruction = `You romanize song lyrics for karaoke display.

You receive a JSON array of lyric lines in Japanese or Korean. Return a JSON
array of the same length where each line is romanized:
- Japanese → Hepburn romaji, spaced by word.
- Korean → Revised Romanization, spaced by word.
- Words already in Latin script (English/Spanish inside mixed lines) pass through unchanged.
- Keep the meaning-free fidelity of a transliteration: do NOT translate.
Reply with ONLY the JSON array, no prose, no markdown fence.`

const translateInstruction = `You translate song lyrics line by line for a karaoke display.

You receive the source language and a JSON array of lyric lines. Return a JSON
array of the same length with each line translated into %s. Keep translations
short and singable-line sized; preserve the emotional register of the lyric.
Reply with ONLY the JSON array, no prose, no markdown fence.`

// Crew implements the pipeline's Agents port with three ADK agents.
type Crew struct {
	model model.LLM
}

func NewCrew(ctx context.Context, apiKey string) (*Crew, error) {
	m, err := gemini.NewModel(ctx, modelName, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("creating gemini model: %w", err)
	}
	return &Crew{model: m}, nil
}

// DetectLanguage returns ja/ko/es/en, or "" when unknown/unsupported —
// letting the caller fall back to deterministic title-script detection.
func (c *Crew) DetectLanguage(ctx context.Context, title, description string) (string, error) {
	desc := description
	if len(desc) > 600 {
		desc = desc[:600]
	}
	out, err := c.runAgent(ctx, "language_detective", detectInstruction,
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
func (c *Crew) Romanize(ctx context.Context, language string, texts []string) ([]string, error) {
	return c.mapLines(ctx, "romanizer", romanizeInstruction,
		"Language: "+language, texts)
}

// Translate renders each lyric line in the target language: Spanish for
// most songs, English when the song is already in Spanish.
func (c *Crew) Translate(ctx context.Context, sourceLang string, texts []string) ([]string, error) {
	target := "Latin American Spanish"
	if sourceLang == "es" {
		target = "English"
	}
	return c.mapLines(ctx, "translator", fmt.Sprintf(translateInstruction, target),
		"Source language: "+sourceLang, texts)
}

// mapLines sends texts through an agent in chunks, expecting a JSON
// array of the same length back for each chunk.
func (c *Crew) mapLines(ctx context.Context, name, instruction, header string, texts []string) ([]string, error) {
	out := make([]string, 0, len(texts))
	for start := 0; start < len(texts); start += chunkSize {
		end := min(start+chunkSize, len(texts))
		chunk := texts[start:end]

		payload, _ := json.Marshal(chunk)
		reply, err := c.runAgent(ctx, name, instruction, header+"\nLines:\n"+string(payload))
		if err != nil {
			return nil, err
		}
		mapped, err := parseStringArray(reply, len(chunk))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		out = append(out, mapped...)
	}
	return out, nil
}

// runAgent executes one single-turn agent with a fresh session.
func (c *Crew) runAgent(ctx context.Context, name, instruction, prompt string) (string, error) {
	a, err := llmagent.New(llmagent.Config{
		Name:        name,
		Model:       c.model,
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
