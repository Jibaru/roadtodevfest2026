package agents

import (
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
)

// ScanArgs is what the model asks for; ScanResult is what it gets back.
// The JSON schema is inferred from these structs by reflection.
type ScanArgs struct {
	Count int `json:"count"` // how many recent crowd words to read
}

type ScanResult struct {
	Words []string `json:"words"`
}

// NewCrowdScannerTool gives a battler eyes on the live audience: it
// returns the most recent words the crowd has shouted through their
// browsers, so the next verse can weave them in.
//
// LIVE-CODING MOMENT: this tool exists but is not wired to any agent.
// To arm it, in NewCrew pass it to MC Gopher:
//
//	scanner, _ := NewCrowdScannerTool(func(n int) []string { return crew.CrowdWords(n) })
//	rt, err := crew.newBattler(ctx, name, instruction, []tool.Tool{scanner})
func NewCrowdScannerTool(recentWords func(n int) []string) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "scan_the_crowd",
		Description: "Read the most recent words the live audience is shouting right now. Use them to spice up your verse.",
	}, func(_ agent.Context, args ScanArgs) (ScanResult, error) {
		n := args.Count
		if n <= 0 || n > 20 {
			n = 5
		}
		return ScanResult{Words: recentWords(n)}, nil
	})
}
