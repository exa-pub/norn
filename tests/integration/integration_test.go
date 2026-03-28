//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/exa-pub/norn/internal/entity"
	"github.com/exa-pub/norn/internal/pkg/devcontainer"
	"github.com/exa-pub/norn/internal/service/agent"
	"github.com/exa-pub/norn/internal/service/instance"
	"github.com/exa-pub/norn/internal/service/storage"
	"github.com/exa-pub/norn/internal/service/terminal"
	"github.com/exa-pub/norn/internal/service/tty"
	"github.com/exa-pub/norn/pkg/dockerutils"
)

var (
	instanceSvc instance.Service
	terminalSvc terminal.Service
	agentSvc    agent.Service
	ttyMgr      tty.Manager
	storageDir  string
)

func TestMain(m *testing.M) {
	// Resolve paths relative to this test file.
	root, err := filepath.Abs("../..")
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve root: %v\n", err)
		os.Exit(1)
	}
	workspaceFolder := filepath.Join(root, "examples", "simple")

	// Temp NornHome.
	storageDir, err = os.MkdirTemp("", "norn-integration-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "mkdtemp: %v\n", err)
		os.Exit(1)
	}

	store := storage.NewFileStore(storageDir)
	dk, err := dockerutils.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "docker: %v\n", err)
		os.Exit(1)
	}
	dc := devcontainer.New()

	dcOpts := &devcontainer.GlobalOptions{
		WorkspaceFolder: workspaceFolder,
	}

	instanceSvc = instance.NewService(store, store, dc, dk, dcOpts)
	ttyMgr = tty.NewManager(dc, dcOpts)

	terminalSvc = terminal.NewService(ttyMgr)
	agentSvc = agent.NewService(store, store, ttyMgr, dk)

	code := m.Run()

	// Cleanup.
	instanceSvc.Shutdown()
	_ = os.RemoveAll(storageDir)

	os.Exit(code)
}

// --- helpers ---

func uniqueName(t *testing.T) string {
	t.Helper()
	// Instance names must match ^[a-z0-9][a-z0-9\-]{0,62}$
	return fmt.Sprintf("t-%d", time.Now().UnixNano()%1000000)
}

func mustCreate(t *testing.T, name string) *entity.Instance {
	t.Helper()
	inst, err := instanceSvc.Create(context.Background(), name)
	if err != nil {
		t.Fatalf("Create(%q): %v", name, err)
	}
	return inst
}

func mustStart(t *testing.T, name string) *entity.Instance {
	t.Helper()
	mustCreate(t, name)

	inst, err := instanceSvc.Start(context.Background(), name, instance.StartOptions{RemoveExisting: true})
	if err != nil {
		t.Fatalf("Start(%q): %v", name, err)
	}

	// Wait for Running.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	for {
		got, err := instanceSvc.Get(ctx, name)
		if err != nil {
			t.Fatalf("Get(%q) while waiting: %v", name, err)
		}
		if got.Status == entity.StatusRunning {
			return got
		}
		if got.Status == entity.StatusError {
			t.Fatalf("Instance %q entered error state: %s", name, got.ErrorMessage)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("Timeout waiting for %q to become Running (last status: %s)", name, got.Status)
		case <-time.After(2 * time.Second):
		}
	}

	return inst
}

func cleanup(t *testing.T, name string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Best-effort: stop then delete.
	_, _ = instanceSvc.Stop(ctx, name)
	_ = instanceSvc.Delete(ctx, name)
}
