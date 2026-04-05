package daemon

import (
	"context"
	"testing"
	"time"

	chttp "connectrpc.com/connect"

	terminalspb "github.com/exa-pub/norn/internal/gen/norn/daemon/terminals/v1"
)

func TestDaemonTerminalCreate(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := terminalClient.Create(ctx, chttp.NewRequest(&terminalspb.CreateRequest{
		Name: "test-terminal",
	}))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	term := resp.Msg.Terminal
	if term.Id == "" {
		t.Fatal("expected non-empty terminal ID")
	}
	if term.Name != "test-terminal" {
		t.Errorf("expected name 'test-terminal', got %q", term.Name)
	}
	if term.TtyId == "" {
		t.Error("expected non-empty TTY ID")
	}

	// Cleanup.
	_, _ = terminalClient.Delete(ctx, chttp.NewRequest(&terminalspb.DeleteRequest{TerminalId: term.Id}))
}

func TestDaemonTerminalDelete(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := terminalClient.Create(ctx, chttp.NewRequest(&terminalspb.CreateRequest{
		Name: "del-terminal",
	}))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = terminalClient.Delete(ctx, chttp.NewRequest(&terminalspb.DeleteRequest{
		TerminalId: resp.Msg.Terminal.Id,
	}))
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Should be gone from list.
	list, err := terminalClient.List(ctx, chttp.NewRequest(&terminalspb.ListRequest{}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, term := range list.Msg.Terminals {
		if term.Id == resp.Msg.Terminal.Id {
			t.Fatal("terminal still present after delete")
		}
	}
}

func TestDaemonTerminalRename(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := terminalClient.Create(ctx, chttp.NewRequest(&terminalspb.CreateRequest{
		Name: "rename-orig",
	}))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer terminalClient.Delete(ctx, chttp.NewRequest(&terminalspb.DeleteRequest{TerminalId: resp.Msg.Terminal.Id}))

	renamed, err := terminalClient.Rename(ctx, chttp.NewRequest(&terminalspb.RenameRequest{
		TerminalId: resp.Msg.Terminal.Id,
		Name:       "rename-new",
	}))
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if renamed.Msg.Terminal.Name != "rename-new" {
		t.Errorf("expected 'rename-new', got %q", renamed.Msg.Terminal.Name)
	}
}

func TestDaemonTerminalList(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c1, err := terminalClient.Create(ctx, chttp.NewRequest(&terminalspb.CreateRequest{Name: "list-t1"}))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer terminalClient.Delete(ctx, chttp.NewRequest(&terminalspb.DeleteRequest{TerminalId: c1.Msg.Terminal.Id}))

	c2, err := terminalClient.Create(ctx, chttp.NewRequest(&terminalspb.CreateRequest{Name: "list-t2"}))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer terminalClient.Delete(ctx, chttp.NewRequest(&terminalspb.DeleteRequest{TerminalId: c2.Msg.Terminal.Id}))

	list, err := terminalClient.List(ctx, chttp.NewRequest(&terminalspb.ListRequest{}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	found := 0
	for _, term := range list.Msg.Terminals {
		if term.Id == c1.Msg.Terminal.Id || term.Id == c2.Msg.Terminal.Id {
			found++
		}
	}
	if found != 2 {
		t.Errorf("expected 2 terminals in list, found %d (total: %d)", found, len(list.Msg.Terminals))
	}
}

func TestDaemonTerminalCustomCmd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := terminalClient.Create(ctx, chttp.NewRequest(&terminalspb.CreateRequest{
		Name: "short-lived",
		Cmd:  []string{"echo", "hello"},
	}))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// The process exits quickly. Wait for auto-cleanup.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		list, err := terminalClient.List(ctx, chttp.NewRequest(&terminalspb.ListRequest{}))
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		found := false
		for _, term := range list.Msg.Terminals {
			if term.Id == resp.Msg.Terminal.Id {
				found = true
				break
			}
		}
		if !found {
			return // success — auto-cleaned up
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("terminal not cleaned up after process exit")
}
