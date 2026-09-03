package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/jibaru/rapbattle/internal/battle/domain"
	"github.com/jibaru/rapbattle/internal/battle/service"
)

// Room separates the two client kinds.
type Room string

const (
	RoomAudience Room = "audience"
	RoomStage    Room = "stage"
)

// GameCommands is what the hub needs from the battle service to route
// inbound audience messages.
type GameCommands interface {
	SubmitTopic(ctx context.Context, clientID, word string) error
	Vote(ctx context.Context, clientID string, battler domain.Battler) error
	AddCrowdWord(ctx context.Context, word string) error
	State(ctx context.Context) (domain.BattleState, error)
}

// Hub fans events out to connected WebSocket clients. Slow clients are
// dropped rather than allowed to stall the show.
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]struct{}
	game    GameCommands
	log     *slog.Logger
}

func NewHub(log *slog.Logger) *Hub {
	return &Hub{
		clients: map[*Client]struct{}{},
		log:     log,
	}
}

// SetGame wires the battle service in after construction (the service
// needs the hub as Broadcaster; the hub needs the service for commands).
func (h *Hub) SetGame(game GameCommands) {
	h.game = game
}

// ToAudience implements service.Broadcaster.
func (h *Hub) ToAudience(event service.Event) { h.broadcast(RoomAudience, event) }

// ToStage implements service.Broadcaster.
func (h *Hub) ToStage(event service.Event) { h.broadcast(RoomStage, event) }

// AudienceCount returns the number of connected audience clients.
func (h *Hub) AudienceCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	n := 0
	for c := range h.clients {
		if c.room == RoomAudience {
			n++
		}
	}
	return n
}

func (h *Hub) broadcast(room Room, event service.Event) {
	data, err := json.Marshal(event)
	if err != nil {
		h.log.Error("marshal event", "error", err)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if c.room != room {
			continue
		}
		select {
		case c.send <- data:
		default:
			// Client can't keep up; close it, the browser will reconnect.
			go c.close()
		}
	}
}

func (h *Hub) add(c *Client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	n := len(h.clients)
	h.mu.Unlock()
	h.log.Info("client connected", "room", c.room, "client_id", c.id, "total", n)
}

func (h *Hub) remove(c *Client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

// inboundMessage is what audience/stage pages send us.
type inboundMessage struct {
	Type    string `json:"type"` // submit_topic | vote | crowd_word
	Word    string `json:"word,omitempty"`
	Battler string `json:"battler,omitempty"`
}

// handleInbound routes one client message to the game. Errors are sent
// back to that client only (e.g. "already voted").
func (h *Hub) handleInbound(c *Client, raw []byte) {
	var msg inboundMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}
	ctx := context.Background()

	var err error
	switch msg.Type {
	case "submit_topic":
		err = h.game.SubmitTopic(ctx, c.id, msg.Word)
	case "vote":
		err = h.game.Vote(ctx, c.id, domain.Battler(msg.Battler))
	case "crowd_word":
		err = h.game.AddCrowdWord(ctx, msg.Word)
	default:
		return
	}
	if err != nil {
		c.sendEvent(service.Event{Type: "error", Payload: err.Error()})
	}
}

// sendSnapshot pushes the current state to a newly connected client.
func (h *Hub) sendSnapshot(c *Client) {
	state, err := h.game.State(context.Background())
	if err != nil {
		state = domain.BattleState{Phase: domain.PhaseIdle}
	}
	c.sendEvent(service.Event{Type: service.EventState, Payload: state})
}
