// Command agenttest diagnoses the agent mapping against a real video,
// printing only errors and counts (never lyric content).
// Dev tool: OPENAI_API_KEY=... go run ./cmd/agenttest <youtubeID>
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jibaru/s1ngo/internal/video/domain"
	"github.com/jibaru/s1ngo/internal/video/infra/agents"
	"github.com/jibaru/s1ngo/internal/video/infra/youtube"
)

func main() {
	id := os.Args[1]
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	f := &youtube.Fetcher{Bin: os.Getenv("YTDLP_PATH")}
	res, err := f.Fetch(ctx, id, nil)
	if err != nil {
		fmt.Println("FETCH ERROR:", err)
		os.Exit(1)
	}
	cues := domain.CleanCues(res.Cues)
	texts := make([]string, len(cues))
	for i, c := range cues {
		texts[i] = c.Text
	}
	fmt.Printf("track=%s cues=%d needsRomanization=%v\n",
		res.SubtitleLang, len(texts), domain.NeedsRomanization(res.SubtitleLang))

	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-6-luna"
	}
	lyricsAgents, err := agents.NewOpenAIAgents(ctx, model, os.Getenv("OPENAI_API_KEY"))
	if err != nil {
		fmt.Println("AGENTS ERROR:", err)
		os.Exit(1)
	}

	if domain.NeedsRomanization(res.SubtitleLang) {
		out, err := lyricsAgents.Romanize(ctx, res.SubtitleLang, texts)
		if err != nil {
			fmt.Println("ROMANIZE ERROR:", err)
		} else {
			ascii := 0
			for _, s := range out {
				ok := true
				for _, c := range s {
					if c > 127 {
						ok = false
						break
					}
				}
				if ok {
					ascii++
				}
			}
			fmt.Printf("ROMANIZE OK: %d lines, %d pure-ASCII\n", len(out), ascii)
		}
	}

	tr, err := lyricsAgents.Translate(ctx, res.SubtitleLang, texts)
	if err != nil {
		fmt.Println("TRANSLATE ERROR:", err)
	} else {
		nonEmpty := 0
		for _, s := range tr {
			if s != "" {
				nonEmpty++
			}
		}
		fmt.Printf("TRANSLATE OK: %d lines, %d non-empty\n", len(tr), nonEmpty)
	}
}
