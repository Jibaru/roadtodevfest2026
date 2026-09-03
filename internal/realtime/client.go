package realtime

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/jibaru/rapbattle/internal/battle/service"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 45 * time.Second
	maxMessageSize = 512
	sendBuffer     = 64
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Same-origin in practice; the page is served by this same binary.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Client is one connected browser.
type Client struct {
	id        string
	room      Room
	conn      *websocket.Conn
	send      chan []byte
	closeOnce sync.Once
}

// ServeWS upgrades an HTTP request into a hub-managed WebSocket client.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, clientID string, room Room) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error("ws upgrade", "error", err)
		return
	}
	c := &Client{
		id:   clientID,
		room: room,
		conn: conn,
		send: make(chan []byte, sendBuffer),
	}
	h.add(c)
	go c.writePump()
	go c.readPump(h)
	h.sendSnapshot(c)
}

func (c *Client) close() {
	c.closeOnce.Do(func() {
		close(c.send)
		_ = c.conn.Close()
	})
}

func (c *Client) sendEvent(event service.Event) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
	}
}

func (c *Client) readPump(h *Hub) {
	defer func() {
		h.remove(c)
		c.close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		h.handleInbound(c, raw)
	}
}

func (c *Client) writePump() {
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
