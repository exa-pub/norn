package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"

	"github.com/exa-pub/norn/internal/service/tty"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// resizeMsg is sent by the client to resize the PTY.
type resizeMsg struct {
	Type string `json:"type"` // "resize"
	Cols uint   `json:"cols"`
	Rows uint   `json:"rows"`
}

// Handler returns an http.Handler that bridges WebSocket ↔ PTY.
// Route: /ws/{session_id}
func Handler(mgr tty.Manager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract session ID from path: /ws/{session_id}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/ws/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			http.Error(w, "missing session_id", http.StatusBadRequest)
			return
		}
		sessionID := parts[0]

		stream, err := mgr.Attach(sessionID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		defer stream.Detach()

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Send replay buffer so the client sees previous output.
		if len(stream.Replay) > 0 {
			if wErr := conn.WriteMessage(websocket.BinaryMessage, stream.Replay); wErr != nil {
				return
			}
		}

		// PTY → WebSocket (live data via subscriber pipe)
		done := make(chan struct{})
		go func() {
			defer close(done)
			buf := make([]byte, 4096)
			for {
				n, err := stream.Read(buf)
				if n > 0 {
					if wErr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); wErr != nil {
						return
					}
				}
				if err != nil {
					return
				}
			}
		}()

		// WebSocket → PTY
		for {
			msgType, data, err := conn.ReadMessage()
			if err != nil {
				break
			}

			// Check for resize message (JSON text).
			if msgType == websocket.TextMessage {
				var msg resizeMsg
				if json.Unmarshal(data, &msg) == nil && msg.Type == "resize" {
					if resizeErr := stream.Resize(msg.Cols, msg.Rows); resizeErr != nil {
						log.Printf("resize %s: %v", sessionID, resizeErr)
					}
					continue
				}
			}

			// Regular input → PTY stdin.
			if _, wErr := stream.Write(data); wErr != nil {
				break
			}
		}

		<-done
	})
}
