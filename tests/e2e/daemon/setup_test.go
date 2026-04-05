package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	dagentsv1connect "github.com/exa-pub/norn/internal/gen/norn/daemon/agents/v1/agentsv1connect"
	dterminalsv1connect "github.com/exa-pub/norn/internal/gen/norn/daemon/terminals/v1/terminalsv1connect"
	"github.com/exa-pub/norn/internal/gen/norn/daemon/tty/v1/ttyv1connect"
	"github.com/exa-pub/norn/tests/e2e/testutil"
)

var (
	agentClient    dagentsv1connect.AgentServiceClient
	terminalClient dterminalsv1connect.TerminalServiceClient
	ttyClient      ttyv1connect.TTYServiceClient
)

func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	mockDir, err := filepath.Abs("testdata")
	if err != nil {
		fmt.Fprintf(os.Stderr, "abs testdata: %v\n", err)
		os.Exit(1)
	}

	result, err := testutil.StartNornDaemon(ctx, mockDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	// Create ConnectRPC clients over Unix socket.
	httpClient := testutil.UnixHTTPClient(result.SocketPath)
	baseURL := "http://localhost/connect"
	agentClient = dagentsv1connect.NewAgentServiceClient(httpClient, baseURL)
	terminalClient = dterminalsv1connect.NewTerminalServiceClient(httpClient, baseURL)
	ttyClient = ttyv1connect.NewTTYServiceClient(httpClient, baseURL)

	code := m.Run()

	// Cleanup.
	_ = result.Container.Terminate(context.Background())
	_ = os.RemoveAll(result.RunDir)

	os.Exit(code)
}
