//go:build integration

package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/exa-pub/norn/internal/entity"
)

func TestTerminalCreateClose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	name := uniqueName(t)
	defer cleanup(t, name)

	mustStart(t, name)

	// Create terminal.
	term, err := terminalSvc.Create(context.Background(), name, "test-shell")
	if err != nil {
		t.Fatalf("Create terminal: %v", err)
	}
	if term.ID == "" || term.TTYID == "" {
		t.Fatal("expected non-empty terminal ID and TTY ID")
	}
	if term.InstanceName != name {
		t.Errorf("expected instance %q, got %q", name, term.InstanceName)
	}

	// Get.
	got, err := terminalSvc.Get(term.ID)
	if err != nil {
		t.Fatalf("Get terminal: %v", err)
	}
	if got.TTYID != term.TTYID {
		t.Errorf("TTY ID mismatch: %q vs %q", got.TTYID, term.TTYID)
	}

	// List.
	list := terminalSvc.List(name)
	if len(list) != 1 {
		t.Fatalf("expected 1 terminal, got %d", len(list))
	}

	// Close.
	if err := terminalSvc.Close(term.ID); err != nil {
		t.Fatalf("Close terminal: %v", err)
	}

	// Should be gone.
	_, err = terminalSvc.Get(term.ID)
	if !errors.Is(err, entity.ErrNotFound) {
		t.Errorf("expected NotFound after close, got %v", err)
	}
}

func TestTerminalTTYReadWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	name := uniqueName(t)
	defer cleanup(t, name)

	mustStart(t, name)

	term, err := terminalSvc.Create(context.Background(), name, "rw-test")
	if err != nil {
		t.Fatalf("Create terminal: %v", err)
	}
	defer terminalSvc.Close(term.ID)

	stream, err := ttyMgr.Attach(term.TTYID)
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}

	// Give shell time to start.
	time.Sleep(1 * time.Second)

	// Write a command.
	_, err = stream.Write([]byte("echo NORN_TEST_OK\n"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Read output.
	buf := make([]byte, 4096)
	deadline := time.Now().Add(10 * time.Second)
	var output string
	for time.Now().Before(deadline) {
		n, err := stream.Read(buf)
		if n > 0 {
			output += string(buf[:n])
		}
		if strings.Contains(output, "NORN_TEST_OK") {
			return // success
		}
		if err != nil {
			break
		}
	}

	t.Fatalf("expected NORN_TEST_OK in output, got: %s", output)
}

func TestTerminalExitCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	name := uniqueName(t)
	defer cleanup(t, name)

	mustStart(t, name)

	term, err := terminalSvc.Create(context.Background(), name, "exit-test")
	if err != nil {
		t.Fatalf("Create terminal: %v", err)
	}

	stream, err := ttyMgr.Attach(term.TTYID)
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}

	// Give shell time to start, then exit.
	time.Sleep(500 * time.Millisecond)
	_, _ = stream.Write([]byte("exit\n"))

	// Wait for onClose to clean up terminal.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		_, err := terminalSvc.Get(term.ID)
		if errors.Is(err, entity.ErrNotFound) {
			return // success — cleaned up
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Fatal("terminal was not cleaned up after exit")
}
