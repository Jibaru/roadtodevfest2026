package service

// Event is a realtime message pushed to clients over WebSocket.
type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

// Event types.
const (
	EventState       = "state"        // full session state (audience + stage)
	EventAgentStatus = "agent_status" // a reviewer's live progress
	EventError       = "error"
)

// AgentStatusPayload reports a reviewer's live progress.
type AgentStatusPayload struct {
	Reviewer string `json:"reviewer"`
	Status   string `json:"status"` // free text: "reading main.go", "done: 4 findings", ...
	Done     bool   `json:"done"`
}
