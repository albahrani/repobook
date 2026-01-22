package watch

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestHub_Broadcast(t *testing.T) {
	h := NewHub()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", h.ServeWS)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = c.Close() }()

	h.Broadcast(Event{Type: "file-changed", Path: "README.md"})
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(msg), "file-changed") {
		t.Fatalf("expected event type in message, got %s", string(msg))
	}
}
