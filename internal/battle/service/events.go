package service

// Event is a realtime message pushed to clients over WebSocket.
type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

// Event types.
const (
	EventState          = "state"           // full battle state (audience + stage)
	EventAgentStatus    = "agent_status"    // a battler started/finished writing
	EventPerformance    = "performance"     // stage only: verse + base64 WAV audio
	EventTTSUnavailable = "tts_unavailable" // stage only: perform it yourself
)

// AgentStatusPayload reports a battler's writing progress.
type AgentStatusPayload struct {
	Battler string `json:"battler"`
	Status  string `json:"status"` // writing | ready | fallback
}

// PerformancePayload carries a performance to the stage.
type PerformancePayload struct {
	Battler  string `json:"battler"`
	Verse    string `json:"verse"`
	AudioB64 string `json:"audio_b64,omitempty"` // WAV, base64; empty if TTS failed
}
