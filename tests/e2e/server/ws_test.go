package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	chttp "connectrpc.com/connect"
	"github.com/gorilla/websocket"

	terminalsv1 "github.com/exa-pub/norn/internal/gen/norn/server/terminals/v1"
)

func (s *ServerSuite) TestWebSocketTTY() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := s.terminals.Create(ctx, chttp.NewRequest(&terminalsv1.CreateRequest{
		InstanceName: s.sharedInstance, Name: "ws-test",
	}))
	s.Require().NoError(err)
	defer s.terminals.Delete(ctx, chttp.NewRequest(&terminalsv1.DeleteRequest{
		InstanceName: s.sharedInstance, Id: resp.Msg.Terminal.Id,
	}))
	ttyID := resp.Msg.Terminal.TtyId

	wsBase := strings.TrimSuffix(s.baseURL, "/connect")
	wsBase = strings.Replace(wsBase, "http://", "ws://", 1)
	wsURL := fmt.Sprintf("%s/ws/%s/%s", wsBase, s.sharedInstance, ttyID)

	header := http.Header{}
	header.Add("Cookie", fmt.Sprintf("norn_secret=%s", testSecret))

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header)
	s.Require().NoError(err)
	defer conn.Close()

	time.Sleep(1 * time.Second)

	s.Require().NoError(conn.WriteMessage(websocket.TextMessage, []byte("echo WS_E2E_OK\n")))

	deadline := time.Now().Add(10 * time.Second)
	conn.SetReadDeadline(deadline)

	var output string
	for time.Now().Before(deadline) {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		output += string(msg)
		if strings.Contains(output, "WS_E2E_OK") {
			return
		}
	}
	s.Fail("expected WS_E2E_OK in output", output)
}

func (s *ServerSuite) TestWebSocketResize() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := s.terminals.Create(ctx, chttp.NewRequest(&terminalsv1.CreateRequest{
		InstanceName: s.sharedInstance, Name: "ws-resize",
	}))
	s.Require().NoError(err)
	defer s.terminals.Delete(ctx, chttp.NewRequest(&terminalsv1.DeleteRequest{
		InstanceName: s.sharedInstance, Id: resp.Msg.Terminal.Id,
	}))
	ttyID := resp.Msg.Terminal.TtyId

	wsBase := strings.TrimSuffix(s.baseURL, "/connect")
	wsBase = strings.Replace(wsBase, "http://", "ws://", 1)
	wsURL := fmt.Sprintf("%s/ws/%s/%s", wsBase, s.sharedInstance, ttyID)

	header := http.Header{}
	header.Add("Cookie", fmt.Sprintf("norn_secret=%s", testSecret))

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header)
	s.Require().NoError(err)
	defer conn.Close()

	time.Sleep(500 * time.Millisecond)

	s.Require().NoError(conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"resize","cols":80,"rows":24}`)))
	s.Require().NoError(conn.WriteMessage(websocket.TextMessage, []byte("echo AFTER_RESIZE\n")))

	deadline := time.Now().Add(5 * time.Second)
	conn.SetReadDeadline(deadline)

	var output string
	for time.Now().Before(deadline) {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		output += string(msg)
		if strings.Contains(output, "AFTER_RESIZE") {
			return
		}
	}
	s.Fail("expected AFTER_RESIZE in output", output)
}
