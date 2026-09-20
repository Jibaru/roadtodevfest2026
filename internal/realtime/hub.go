// Package realtime is a push-only WebSocket fan-out: the pipeline
// broadcasts video updates and progress; browsers just listen. Slow
// clients are dropped rather than allowed to stall the show.
package realtime

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 45 * time.Second
	sendBuffer = 64
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Same-origin in practice; the page is served by this same binary.
	CheckOrigin: func(r *http.Request) bool { return true },
}

type event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

// Hub tracks connected clients and fans events out to all of them.
type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
	log     *slog.Logger
}

func NewHub(log *slog.Logger) *Hub {
	return &Hub{clients: map[*client]struct{}{}, log: log}
}

// Broadcast implements the pipeline's Broadcaster port.
func (h *Hub) Broadcast(eventType string, payload any) {
	data, err := json.Marshal(event{Type: eventType, Payload: payload})
	if err != nil {
		h.log.Error("marshal event", "error", err)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- data:
		default:
			go c.close() // can't keep up; the browser will reconnect
		}
	}
}

// ClientCount returns the number of connected browsers.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ServeWS upgrades an HTTP request into a hub-managed connection.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error("ws upgrade", "error", err)
		return
	}
	c := &client{conn: conn, send: make(chan []byte, sendBuffer)}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	n := len(h.clients)
	h.mu.Unlock()
	h.log.Info("ws client connected", "total", n)

	go c.writePump()
	go func() {
		c.readPump()
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
		c.close()
	}()
}

type client struct {
	conn      *websocket.Conn
	send      chan []byte
	closeOnce sync.Once
}

func (c *client) close() {
	c.closeOnce.Do(func() {
		close(c.send)
		_ = c.conn.Close()
	})
}

// readPump discards inbound messages (the UI talks over REST) but keeps
// the pong handler alive.
func (c *client) readPump() {
	c.conn.SetReadLimit(512)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (c *client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case data, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
