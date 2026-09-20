// Command fetchtest exercises the real yt-dlp integration against a
// live YouTube video. Dev tool only: go run ./cmd/fetchtest <id>
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jibaru/s1ngo/internal/lines"
	"github.com/jibaru/s1ngo/internal/ytdlp"
)

func main() {
	id := "dQw4w9WgXcQ"
	if len(os.Args) > 1 {
		id = os.Args[1]
	}
	f := &ytdlp.Fetcher{Bin: os.Getenv("YTDLP_PATH")}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	res, err := f.Fetch(ctx, id, nil)
	if err != nil {
		fmt.Println("FETCH ERROR:", err)
		os.Exit(1)
	}
	fmt.Printf("title=%q dur=%ds declared=%q subLang=%q auto=%v cues=%d\n",
		res.Title, res.DurationSec, res.DeclaredLanguage, res.SubtitleLang, res.IsAuto, len(res.Cues))
	cues := lines.CleanCues(res.Cues)
	built := lines.Build(cues, nil, nil)
	fmt.Println("lines:", len(built))
	for i, l := range built[:min(4, len(built))] {
		fmt.Printf("  [%d] %d-%dms:", i, l.StartMs, l.EndMs)
		for _, w := range l.Words {
			fmt.Printf(" %s(%d-%d)", w.Text, w.StartMs, w.EndMs)
		}
		fmt.Println()
	}
}
