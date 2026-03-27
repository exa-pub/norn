//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/exa-pub/norn/internal/entity"
	"github.com/exa-pub/norn/internal/service/instance"
)

func TestInstanceCreate(t *testing.T) {
	name := uniqueName(t)
	defer cleanup(t, name)

	inst, err := instanceSvc.Create(context.Background(), name)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if inst.Status != entity.StatusStopped {
		t.Errorf("expected Stopped, got %s", inst.Status)
	}
	if inst.ID == "" {
		t.Error("expected non-empty ID")
	}

	// Duplicate should fail.
	_, err = instanceSvc.Create(context.Background(), name)
	if !errors.Is(err, entity.ErrAlreadyExists) {
		t.Errorf("expected AlreadyExists, got %v", err)
	}
}

func TestInstanceCreateInvalidName(t *testing.T) {
	_, err := instanceSvc.Create(context.Background(), "INVALID_NAME!")
	if !errors.Is(err, entity.ErrInvalidName) {
		t.Errorf("expected InvalidName, got %v", err)
	}
}

func TestInstanceStartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	name := uniqueName(t)
	defer cleanup(t, name)

	inst := mustStart(t, name)
	if inst.DockerID == "" {
		t.Error("expected non-empty DockerID after running")
	}

	// Stop.
	stopped, err := instanceSvc.Stop(context.Background(), name)
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if stopped.Status != entity.StatusStopped {
		t.Errorf("expected Stopped after stop, got %s", stopped.Status)
	}
}

func TestInstanceDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	name := uniqueName(t)

	mustStart(t, name)

	// Delete should stop container and remove everything.
	err := instanceSvc.Delete(context.Background(), name)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Get should return NotFound.
	_, err = instanceSvc.Get(context.Background(), name)
	if !errors.Is(err, entity.ErrNotFound) {
		t.Errorf("expected NotFound after delete, got %v", err)
	}
}

func TestInstanceLogs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	name := uniqueName(t)
	defer cleanup(t, name)

	mustCreate(t, name)

	_, err := instanceSvc.Start(context.Background(), name, instance.StartOptions{RemoveExisting: true})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	ch, err := instanceSvc.WatchLogs(ctx, name)
	if err != nil {
		t.Fatalf("WatchLogs: %v", err)
	}

	count := 0
	for entry := range ch {
		if entry.Line == "" && !entry.IsStderr {
			continue
		}
		count++
		if count >= 3 {
			break // We've seen enough.
		}
	}

	if count == 0 {
		t.Error("expected at least one log entry")
	}
}

func TestInstanceRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	name := uniqueName(t)
	defer cleanup(t, name)

	first := mustStart(t, name)
	firstDockerID := first.DockerID

	_, err := instanceSvc.Stop(context.Background(), name)
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}

	second := mustStart2(t, name)
	if second.DockerID == firstDockerID {
		t.Error("expected different DockerID after restart")
	}
}

// mustStart2 starts an already-created instance and waits for Running.
func mustStart2(t *testing.T, name string) *entity.Instance {
	t.Helper()

	_, err := instanceSvc.Start(context.Background(), name, instance.StartOptions{RemoveExisting: true})
	if err != nil {
		t.Fatalf("Start(%q): %v", name, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	for {
		got, err := instanceSvc.Get(ctx, name)
		if err != nil {
			t.Fatalf("Get(%q): %v", name, err)
		}
		if got.Status == entity.StatusRunning {
			return got
		}
		if got.Status == entity.StatusError {
			t.Fatalf("Instance %q error: %s", name, got.ErrorMessage)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("Timeout waiting for %q Running", name)
		case <-time.After(2 * time.Second):
		}
	}
}
