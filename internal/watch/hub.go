package watch

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type Event struct {
	Type string `json:"type"`
	Path string `json:"path,omitempty"`
}

type Hub struct {
	mu    sync.Mutex
	conns map[*websocket.Conn]struct{}
}

func NewHub() *Hub {
	return &Hub{conns: make(map[*websocket.Conn]struct{})}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Localhost only by default; still allow browser connections.
		return true
	},
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	h.mu.Lock()
	h.conns[c] = struct{}{}
	h.mu.Unlock()

	// Read loop: we don't handle client messages, but this detects disconnects.
	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.conns, c)
			h.mu.Unlock()
			_ = c.Close()
		}()
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

func (h *Hub) Broadcast(ev Event) {
	payload, _ := json.Marshal(ev)
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.conns {
		_ = c.WriteMessage(websocket.TextMessage, payload)
	}
}
