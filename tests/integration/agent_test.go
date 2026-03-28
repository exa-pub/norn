//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/exa-pub/norn/internal/entity"
)

func TestAgentSessionCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	name := uniqueName(t)
	defer cleanup(t, name)

	mustStart(t, name)

	// Create.
	sess, err := agentSvc.CreateSession(context.Background(), name, "my-agent")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.ID == "" {
		t.Fatal("expected non-empty session ID")
	}
	if sess.Name != "my-agent" {
		t.Errorf("expected name 'my-agent', got %q", sess.Name)
	}
	if sess.Running {
		t.Error("expected Running=false after create")
	}

	// Get.
	got, err := agentSvc.GetSession(context.Background(), name, sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.ID != sess.ID {
		t.Errorf("ID mismatch")
	}

	// List.
	list, err := agentSvc.ListSessions(context.Background(), name)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 session, got %d", len(list))
	}

	// Update name.
	updated, err := agentSvc.UpdateSessionName(context.Background(), name, sess.ID, "renamed")
	if err != nil {
		t.Fatalf("UpdateSessionName: %v", err)
	}
	if updated.Name != "renamed" {
		t.Errorf("expected name 'renamed', got %q", updated.Name)
	}

	// Delete.
	if err := agentSvc.DeleteSession(context.Background(), name, sess.ID); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	// Should be gone.
	_, err = agentSvc.GetSession(context.Background(), name, sess.ID)
	if !errors.Is(err, entity.ErrNotFound) {
		t.Errorf("expected NotFound after delete, got %v", err)
	}
}

func TestAgentDeleteWhileRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	name := uniqueName(t)
	defer cleanup(t, name)

	mustStart(t, name)

	sess, err := agentSvc.CreateSession(context.Background(), name, "running-agent")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Launch with a simple long-running command (sleep via claude isn't practical,
	// but we test the precondition check).
	launched, err := agentSvc.Launch(context.Background(), name, sess.ID, "say hello")
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if !launched.Running {
		t.Error("expected Running=true after launch")
	}
	if launched.TTYID == "" {
		t.Error("expected non-empty TTYID after launch")
	}

	// Delete should fail while running.
	err = agentSvc.DeleteSession(context.Background(), name, sess.ID)
	if !errors.Is(err, entity.ErrFailedPrecondition) {
		t.Errorf("expected FailedPrecondition, got %v", err)
	}

	// Stop.
	stopped, err := agentSvc.Stop(context.Background(), name, sess.ID)
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if stopped.Running {
		t.Error("expected Running=false after stop")
	}

	// Now delete should work.
	if err := agentSvc.DeleteSession(context.Background(), name, sess.ID); err != nil {
		t.Fatalf("DeleteSession after stop: %v", err)
	}
}

func TestAgentLaunchRequiresRunningContainer(t *testing.T) {
	name := uniqueName(t)
	defer cleanup(t, name)

	// Create but DON'T start.
	mustCreate(t, name)

	sess, err := agentSvc.CreateSession(context.Background(), name, "no-container")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Launch should fail — no running container.
	_, err = agentSvc.Launch(context.Background(), name, sess.ID, "hello")
	if err == nil {
		t.Fatal("expected error when launching without running container")
	}
}
