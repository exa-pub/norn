package daemon

import (
	"context"
	"strings"
	"testing"
	"time"

	chttp "connectrpc.com/connect"

	terminalspb "github.com/exa-pub/norn/internal/gen/norn/daemon/terminals/v1"
	ttypb "github.com/exa-pub/norn/internal/gen/norn/daemon/tty/v1"
)

// createTerminalForTTY is a helper that creates a bash terminal and returns its TTY ID.
func createTerminalForTTY(t *testing.T, ctx context.Context) (terminalID, ttyID string) {
	t.Helper()
	resp, err := terminalClient.Create(ctx, chttp.NewRequest(&terminalspb.CreateRequest{
		Name: "tty-test",
	}))
	if err != nil {
		t.Fatalf("Create terminal: %v", err)
	}
	return resp.Msg.Terminal.Id, resp.Msg.Terminal.TtyId
}

func TestTTYAttach(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	termID, ttyID := createTerminalForTTY(t, ctx)
	defer terminalClient.Delete(ctx, chttp.NewRequest(&terminalspb.DeleteRequest{TerminalId: termID}))

	stream := ttyClient.Attach(ctx)

	// Send attach message.
	if err := stream.Send(&ttypb.TTYInput{
		Payload: &ttypb.TTYInput_Attach{Attach: &ttypb.TTYInputAttach{TtyId: ttyID}},
	}); err != nil {
		t.Fatalf("Send attach: %v", err)
	}

	// Should receive at least one message (replay or data).
	msg, err := stream.Receive()
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if msg.Payload == nil {
		t.Fatal("expected non-nil payload")
	}

	_ = stream.CloseRequest()
	_ = stream.CloseResponse()
}

func TestTTYReadWrite(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	termID, ttyID := createTerminalForTTY(t, ctx)
	defer terminalClient.Delete(ctx, chttp.NewRequest(&terminalspb.DeleteRequest{TerminalId: termID}))

	stream := ttyClient.Attach(ctx)
	defer func() {
		_ = stream.CloseRequest()
		_ = stream.CloseResponse()
	}()

	// Attach.
	if err := stream.Send(&ttypb.TTYInput{
		Payload: &ttypb.TTYInput_Attach{Attach: &ttypb.TTYInputAttach{TtyId: ttyID}},
	}); err != nil {
		t.Fatalf("Send attach: %v", err)
	}

	// Give shell time to start.
	time.Sleep(500 * time.Millisecond)

	// Write a command.
	if err := stream.Send(&ttypb.TTYInput{
		Payload: &ttypb.TTYInput_Data{Data: &ttypb.TTYInputData{Data: []byte("echo NORN_E2E_OK\n")}},
	}); err != nil {
		t.Fatalf("Send data: %v", err)
	}

	// Read output until we see NORN_E2E_OK.
	deadline := time.Now().Add(10 * time.Second)
	var output string
	for time.Now().Before(deadline) {
		msg, err := stream.Receive()
		if err != nil {
			t.Fatalf("Receive: %v", err)
		}
		switch p := msg.Payload.(type) {
		case *ttypb.TTYOutput_Data:
			output += string(p.Data.Data)
		case *ttypb.TTYOutput_Replay:
			output += string(p.Replay.Data)
		}
		if strings.Contains(output, "NORN_E2E_OK") {
			return // success
		}
	}
	t.Fatalf("expected NORN_E2E_OK in output, got: %s", output)
}

func TestTTYResize(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	termID, ttyID := createTerminalForTTY(t, ctx)
	defer terminalClient.Delete(ctx, chttp.NewRequest(&terminalspb.DeleteRequest{TerminalId: termID}))

	stream := ttyClient.Attach(ctx)
	defer func() {
		_ = stream.CloseRequest()
		_ = stream.CloseResponse()
	}()

	// Attach.
	if err := stream.Send(&ttypb.TTYInput{
		Payload: &ttypb.TTYInput_Attach{Attach: &ttypb.TTYInputAttach{TtyId: ttyID}},
	}); err != nil {
		t.Fatalf("Send attach: %v", err)
	}

	// Resize — should not error.
	if err := stream.Send(&ttypb.TTYInput{
		Payload: &ttypb.TTYInput_Resize{Resize: &ttypb.TTYInputResize{Cols: 80, Rows: 24}},
	}); err != nil {
		t.Fatalf("Send resize: %v", err)
	}

	// Should still be able to receive data after resize.
	time.Sleep(200 * time.Millisecond)
	if err := stream.Send(&ttypb.TTYInput{
		Payload: &ttypb.TTYInput_Data{Data: &ttypb.TTYInputData{Data: []byte("echo RESIZED\n")}},
	}); err != nil {
		t.Fatalf("Send after resize: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	var output string
	for time.Now().Before(deadline) {
		msg, err := stream.Receive()
		if err != nil {
			break
		}
		switch p := msg.Payload.(type) {
		case *ttypb.TTYOutput_Data:
			output += string(p.Data.Data)
		case *ttypb.TTYOutput_Replay:
			output += string(p.Replay.Data)
		}
		if strings.Contains(output, "RESIZED") {
			return // success
		}
	}
	t.Fatalf("expected RESIZED in output after resize, got: %s", output)
}

func TestTTYReplay(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	termID, ttyID := createTerminalForTTY(t, ctx)
	defer terminalClient.Delete(ctx, chttp.NewRequest(&terminalspb.DeleteRequest{TerminalId: termID}))

	// First stream: write some data.
	s1 := ttyClient.Attach(ctx)
	if err := s1.Send(&ttypb.TTYInput{
		Payload: &ttypb.TTYInput_Attach{Attach: &ttypb.TTYInputAttach{TtyId: ttyID}},
	}); err != nil {
		t.Fatalf("s1 attach: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	if err := s1.Send(&ttypb.TTYInput{
		Payload: &ttypb.TTYInput_Data{Data: &ttypb.TTYInputData{Data: []byte("echo REPLAY_MARKER\n")}},
	}); err != nil {
		t.Fatalf("s1 write: %v", err)
	}

	// Wait for command to execute.
	time.Sleep(1 * time.Second)

	_ = s1.CloseRequest()
	_ = s1.CloseResponse()

	// Second stream: should see REPLAY_MARKER in replay buffer.
	s2 := ttyClient.Attach(ctx)
	defer func() {
		_ = s2.CloseRequest()
		_ = s2.CloseResponse()
	}()

	if err := s2.Send(&ttypb.TTYInput{
		Payload: &ttypb.TTYInput_Attach{Attach: &ttypb.TTYInputAttach{TtyId: ttyID}},
	}); err != nil {
		t.Fatalf("s2 attach: %v", err)
	}

	// Read replay + live data.
	deadline := time.Now().Add(5 * time.Second)
	var output string
	for time.Now().Before(deadline) {
		msg, err := s2.Receive()
		if err != nil {
			break
		}
		switch p := msg.Payload.(type) {
		case *ttypb.TTYOutput_Replay:
			output += string(p.Replay.Data)
		case *ttypb.TTYOutput_Data:
			output += string(p.Data.Data)
		}
		if strings.Contains(output, "REPLAY_MARKER") {
			return // success
		}
	}
	t.Fatalf("expected REPLAY_MARKER in replay, got: %s", output)
}

func TestTTYMultipleSubscribers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	termID, ttyID := createTerminalForTTY(t, ctx)
	defer terminalClient.Delete(ctx, chttp.NewRequest(&terminalspb.DeleteRequest{TerminalId: termID}))

	// Attach two streams.
	s1 := ttyClient.Attach(ctx)
	defer func() {
		_ = s1.CloseRequest()
		_ = s1.CloseResponse()
	}()

	s2 := ttyClient.Attach(ctx)
	defer func() {
		_ = s2.CloseRequest()
		_ = s2.CloseResponse()
	}()

	for _, s := range []*struct{ stream interface{ Send(*ttypb.TTYInput) error } }{
		{s1}, {s2},
	} {
		if err := s.stream.Send(&ttypb.TTYInput{
			Payload: &ttypb.TTYInput_Attach{Attach: &ttypb.TTYInputAttach{TtyId: ttyID}},
		}); err != nil {
			t.Fatalf("attach: %v", err)
		}
	}

	time.Sleep(500 * time.Millisecond)

	// Write via stream 1.
	if err := s1.Send(&ttypb.TTYInput{
		Payload: &ttypb.TTYInput_Data{Data: &ttypb.TTYInputData{Data: []byte("echo MULTI_OK\n")}},
	}); err != nil {
		t.Fatalf("s1 write: %v", err)
	}

	// Both streams should see the output.
	for name, stream := range map[string]interface {
		Receive() (*ttypb.TTYOutput, error)
	}{"s1": s1, "s2": s2} {
		deadline := time.Now().Add(5 * time.Second)
		var output string
		found := false
		for time.Now().Before(deadline) {
			msg, err := stream.Receive()
			if err != nil {
				break
			}
			switch p := msg.Payload.(type) {
			case *ttypb.TTYOutput_Data:
				output += string(p.Data.Data)
			case *ttypb.TTYOutput_Replay:
				output += string(p.Replay.Data)
			}
			if strings.Contains(output, "MULTI_OK") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("stream %s: expected MULTI_OK, got: %s", name, output)
		}
	}
}
