package daemon

import (
	"context"
	"testing"
	"time"

	chttp "connectrpc.com/connect"

	agentspb "github.com/exa-pub/norn/internal/gen/norn/daemon/agents/v1"
)

func TestDaemonAgentCreate(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := agentClient.Create(ctx, chttp.NewRequest(&agentspb.CreateRequest{
		Name:   "test-agent",
		Prompt: "hello",
	}))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sess := resp.Msg.Session
	if sess.Id == "" {
		t.Fatal("expected non-empty session ID")
	}
	if sess.Name != "test-agent" {
		t.Errorf("expected name 'test-agent', got %q", sess.Name)
	}
	if !sess.Running {
		t.Error("expected Running=true after create")
	}
	if sess.TtyId == "" {
		t.Error("expected non-empty TTY ID")
	}

	// Cleanup.
	_, _ = agentClient.Delete(ctx, chttp.NewRequest(&agentspb.DeleteRequest{SessionId: sess.Id}))
}

func TestDaemonAgentGet(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	created, err := agentClient.Create(ctx, chttp.NewRequest(&agentspb.CreateRequest{
		Name: "get-agent",
	}))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer agentClient.Delete(ctx, chttp.NewRequest(&agentspb.DeleteRequest{SessionId: created.Msg.Session.Id}))

	got, err := agentClient.Get(ctx, chttp.NewRequest(&agentspb.GetRequest{
		SessionId: created.Msg.Session.Id,
	}))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Msg.Session.Id != created.Msg.Session.Id {
		t.Errorf("ID mismatch: %q vs %q", got.Msg.Session.Id, created.Msg.Session.Id)
	}
	if got.Msg.Session.Name != "get-agent" {
		t.Errorf("expected name 'get-agent', got %q", got.Msg.Session.Name)
	}
}

func TestDaemonAgentList(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c1, err := agentClient.Create(ctx, chttp.NewRequest(&agentspb.CreateRequest{Name: "list-a1"}))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer agentClient.Delete(ctx, chttp.NewRequest(&agentspb.DeleteRequest{SessionId: c1.Msg.Session.Id}))

	c2, err := agentClient.Create(ctx, chttp.NewRequest(&agentspb.CreateRequest{Name: "list-a2"}))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer agentClient.Delete(ctx, chttp.NewRequest(&agentspb.DeleteRequest{SessionId: c2.Msg.Session.Id}))

	list, err := agentClient.List(ctx, chttp.NewRequest(&agentspb.ListRequest{}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	found := 0
	for _, s := range list.Msg.Sessions {
		if s.Id == c1.Msg.Session.Id || s.Id == c2.Msg.Session.Id {
			found++
		}
	}
	if found != 2 {
		t.Errorf("expected 2 sessions in list, found %d (total: %d)", found, len(list.Msg.Sessions))
	}
}

func TestDaemonAgentRename(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	created, err := agentClient.Create(ctx, chttp.NewRequest(&agentspb.CreateRequest{Name: "rename-orig"}))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer agentClient.Delete(ctx, chttp.NewRequest(&agentspb.DeleteRequest{SessionId: created.Msg.Session.Id}))

	renamed, err := agentClient.Rename(ctx, chttp.NewRequest(&agentspb.RenameRequest{
		SessionId: created.Msg.Session.Id,
		Name:      "rename-new",
	}))
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if renamed.Msg.Session.Name != "rename-new" {
		t.Errorf("expected 'rename-new', got %q", renamed.Msg.Session.Name)
	}
}

func TestDaemonAgentStop(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	created, err := agentClient.Create(ctx, chttp.NewRequest(&agentspb.CreateRequest{Name: "stop-agent"}))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer agentClient.Delete(ctx, chttp.NewRequest(&agentspb.DeleteRequest{SessionId: created.Msg.Session.Id}))

	if !created.Msg.Session.Running {
		t.Fatal("expected Running=true after create")
	}

	stopped, err := agentClient.Stop(ctx, chttp.NewRequest(&agentspb.StopRequest{
		SessionId: created.Msg.Session.Id,
	}))
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if stopped.Msg.Session.Running {
		t.Error("expected Running=false after stop")
	}
}

func TestDaemonAgentDelete(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	created, err := agentClient.Create(ctx, chttp.NewRequest(&agentspb.CreateRequest{Name: "del-agent"}))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = agentClient.Delete(ctx, chttp.NewRequest(&agentspb.DeleteRequest{
		SessionId: created.Msg.Session.Id,
	}))
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Get should fail.
	_, err = agentClient.Get(ctx, chttp.NewRequest(&agentspb.GetRequest{
		SessionId: created.Msg.Session.Id,
	}))
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestDaemonAgentStartResume(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	created, err := agentClient.Create(ctx, chttp.NewRequest(&agentspb.CreateRequest{Name: "resume-agent"}))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer agentClient.Delete(ctx, chttp.NewRequest(&agentspb.DeleteRequest{SessionId: created.Msg.Session.Id}))

	// Stop first.
	_, err = agentClient.Stop(ctx, chttp.NewRequest(&agentspb.StopRequest{
		SessionId: created.Msg.Session.Id,
	}))
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// Resume via Start.
	started, err := agentClient.Start(ctx, chttp.NewRequest(&agentspb.StartRequest{
		SessionId: created.Msg.Session.Id,
	}))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !started.Msg.Session.Running {
		t.Error("expected Running=true after resume")
	}
	if started.Msg.Session.TtyId == "" {
		t.Error("expected non-empty TTY ID after resume")
	}
}
