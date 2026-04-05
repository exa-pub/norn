package server

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	chttp "connectrpc.com/connect"
	"github.com/stretchr/testify/suite"

	agentsv1 "github.com/exa-pub/norn/internal/gen/norn/server/agents/v1"
	sagentsv1connect "github.com/exa-pub/norn/internal/gen/norn/server/agents/v1/agentsv1connect"
	containersv1 "github.com/exa-pub/norn/internal/gen/norn/server/containers/v1"
	containersv1connect "github.com/exa-pub/norn/internal/gen/norn/server/containers/v1/containersv1connect"
	sterminalsv1connect "github.com/exa-pub/norn/internal/gen/norn/server/terminals/v1/terminalsv1connect"
	"github.com/exa-pub/norn/tests/e2e/testutil"
)

const testSecret = "e2e-test-secret"

type ServerSuite struct {
	suite.Suite

	containers containersv1connect.ContainerServiceClient
	agents     sagentsv1connect.AgentServiceClient
	terminals  sterminalsv1connect.TerminalServiceClient
	baseURL    string

	// sharedInstance is a devcontainer started once for the entire suite.
	sharedInstance string

	nornResult *testutil.NornContainerResult
}

func TestServerSuite(t *testing.T) {
	suite.Run(t, new(ServerSuite))
}

func (s *ServerSuite) SetupSuite() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	projectRoot, err := filepath.Abs("../../..")
	s.Require().NoError(err)
	workspacePath := filepath.Join(projectRoot, "examples", "test")

	s.nornResult, err = testutil.StartNornContainer(ctx, testSecret, workspacePath)
	s.Require().NoError(err, "start norn container")

	s.baseURL = s.nornResult.BaseURL
	httpClient := testutil.AuthHTTPClient(testutil.H2CClient(), testSecret)
	s.containers = containersv1connect.NewContainerServiceClient(httpClient, s.baseURL)
	s.agents = sagentsv1connect.NewAgentServiceClient(httpClient, s.baseURL)
	s.terminals = sterminalsv1connect.NewTerminalServiceClient(httpClient, s.baseURL)

	// Start a shared devcontainer for agent/terminal/ws tests.
	s.sharedInstance = "shared"
	s.Require().NoError(s.startInstance(ctx, s.sharedInstance), "start shared instance")
}

func (s *ServerSuite) TearDownSuite() {
	if s.nornResult != nil {
		_ = s.nornResult.Container.Terminate(context.Background())
	}
}

// startInstance creates, starts a devcontainer and waits for daemon to be reachable.
func (s *ServerSuite) startInstance(ctx context.Context, name string) error {
	_, err := s.containers.Create(ctx, chttp.NewRequest(&containersv1.CreateRequest{Name: name}))
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	_, err = s.containers.Start(ctx, chttp.NewRequest(&containersv1.StartRequest{Name: name, RemoveExisting: true}))
	if err != nil {
		return fmt.Errorf("start: %w", err)
	}

	for {
		got, err := s.containers.Get(ctx, chttp.NewRequest(&containersv1.GetRequest{Name: name}))
		if err != nil {
			return fmt.Errorf("get: %w", err)
		}
		switch got.Msg.Container.Status {
		case containersv1.ContainerStatus_CONTAINER_STATUS_RUNNING:
			return s.waitForDaemon(ctx, name)
		case containersv1.ContainerStatus_CONTAINER_STATUS_ERROR:
			return fmt.Errorf("error state: %s", got.Msg.Container.ErrorMessage)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (s *ServerSuite) waitForDaemon(ctx context.Context, name string) error {
	for {
		_, err := s.agents.List(ctx, chttp.NewRequest(&agentsv1.ListRequest{InstanceName: name}))
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("daemon not ready: %w", err)
		case <-time.After(1 * time.Second):
		}
	}
}

// mustStartContainer creates a new devcontainer (for container lifecycle tests).
func (s *ServerSuite) mustStartContainer(name string) {
	s.T().Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.startInstance(ctx, name))
}

// cleanupContainer best-effort stops and deletes a container.
func (s *ServerSuite) cleanupContainer(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s.containers.Stop(ctx, chttp.NewRequest(&containersv1.StopRequest{Name: name}))
	s.containers.Delete(ctx, chttp.NewRequest(&containersv1.DeleteRequest{Name: name}))
}

