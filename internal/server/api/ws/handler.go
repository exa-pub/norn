package ws

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	ttypb "github.com/exa-pub/norn/internal/gen/norn/daemon/tty/v1"
	"github.com/exa-pub/norn/internal/server/service/daemonconn"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// resizeMsg is sent by the client to resize the PTY.
type resizeMsg struct {
	Type string `json:"type"` // "resize"
	Cols uint32 `json:"cols"`
	Rows uint32 `json:"rows"`
}

// Handler returns an http.Handler that proxies WebSocket ↔ daemon TTY via gRPC.
// Route: /ws/{instanceName}/{ttyID}
func Handler(pool *daemonconn.Pool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract instanceName and ttyID from path: /ws/{instanceName}/{ttyID}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/ws/"), "/")
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			http.Error(w, "expected /ws/{instanceName}/{ttyID}", http.StatusBadRequest)
			return
		}
		instanceName := parts[0]
		ttyID := parts[1]

		conn, err := pool.Get(instanceName)
		if err != nil {
			zap.L().Warn("ws: daemon unavailable", zap.String("instance", instanceName), zap.Error(err))
			http.Error(w, "daemon unavailable", http.StatusBadGateway)
			return
		}

		// Open bidi stream to daemon
		stream := conn.TTY.Attach(r.Context())

		// First message: attach to specific TTY
		if err := stream.Send(&ttypb.TTYInput{
			Payload: &ttypb.TTYInput_Attach{Attach: &ttypb.TTYInputAttach{TtyId: ttyID}},
		}); err != nil {
			zap.L().Warn("ws: daemon attach failed", zap.String("instance", instanceName), zap.String("ttyID", ttyID), zap.Error(err))
			http.Error(w, "daemon attach failed", http.StatusBadGateway)
			return
		}

		wsConn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			stream.CloseRequest()
			stream.CloseResponse()
			return
		}
		defer wsConn.Close()

		// gRPC stream → WebSocket (daemon output → browser)
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				msg, err := stream.Receive()
				if err != nil {
					return
				}
				switch p := msg.Payload.(type) {
				case *ttypb.TTYOutput_Replay:
					if wErr := wsConn.WriteMessage(websocket.BinaryMessage, p.Replay.Data); wErr != nil {
						return
					}
				case *ttypb.TTYOutput_Data:
					if wErr := wsConn.WriteMessage(websocket.BinaryMessage, p.Data.Data); wErr != nil {
						return
					}
				case *ttypb.TTYOutput_Closed:
					return
				}
			}
		}()

		// WebSocket → gRPC stream (browser input → daemon)
		for {
			msgType, data, err := wsConn.ReadMessage()
			if err != nil {
				break
			}

			if msgType == websocket.TextMessage {
				var msg resizeMsg
				if json.Unmarshal(data, &msg) == nil && msg.Type == "resize" {
					if sendErr := stream.Send(&ttypb.TTYInput{
						Payload: &ttypb.TTYInput_Resize{Resize: &ttypb.TTYInputResize{
							Cols: msg.Cols, Rows: msg.Rows,
						}},
					}); sendErr != nil {
						break
					}
					continue
				}
			}

			if sendErr := stream.Send(&ttypb.TTYInput{
				Payload: &ttypb.TTYInput_Data{Data: &ttypb.TTYInputData{Data: data}},
			}); sendErr != nil {
				break
			}
		}

		stream.CloseRequest()
		<-done
	})
}
