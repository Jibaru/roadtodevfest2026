package agents

import (
	"strings"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"

	"github.com/jibaru/agentarena/internal/repofetch"
)

// SearchArgs / SearchResult: the typed structs are the schema.
type SearchArgs struct {
	Query string `json:"query"` // case-insensitive substring to look for
}

type Match struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

type SearchResult struct {
	Matches []Match `json:"matches"`
}

// NewSearchCodeTool gives a reviewer grep across the whole repository —
// instead of opening files one by one, it can hunt a pattern everywhere
// ("os.Getenv", "TODO", "password") in a single call.
//
// LIVE-CODING MOMENT: this tool exists but is not wired to any agent.
// To arm it, in Crew.Review add it next to read_file:
//
//	search := NewSearchCodeTool(snap, onStatus)
//	Tools: []tool.Tool{readFile, search}
func NewSearchCodeTool(snap *repofetch.Snapshot, onStatus func(string)) tool.Tool {
	t, _ := functiontool.New(functiontool.Config{
		Name:        "search_code",
		Description: "Search a case-insensitive substring across every file in the repository. Returns matching lines with paths.",
	}, func(_ agent.Context, args SearchArgs) (SearchResult, error) {
		onStatus("searching for \"" + args.Query + "\"")
		q := strings.ToLower(args.Query)
		var res SearchResult
		for _, f := range snap.Files {
			for i, line := range strings.Split(f.Content, "\n") {
				if strings.Contains(strings.ToLower(line), q) {
					res.Matches = append(res.Matches, Match{Path: f.Path, Line: i + 1, Text: strings.TrimSpace(line)})
					if len(res.Matches) >= 40 {
						return res, nil
					}
				}
			}
		}
		return res, nil
	})
	return t
}
