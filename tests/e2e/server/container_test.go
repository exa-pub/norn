package server

import (
	"context"
	"time"

	chttp "connectrpc.com/connect"

	containersv1 "github.com/exa-pub/norn/internal/gen/norn/server/containers/v1"
	"github.com/exa-pub/norn/internal/gen/norn/server/containers/v1/containersv1connect"
	"github.com/exa-pub/norn/tests/e2e/testutil"
)

func newContainerCreateReq(name string) *chttp.Request[containersv1.CreateRequest] {
	return chttp.NewRequest(&containersv1.CreateRequest{Name: name})
}
func newContainerStartReq(name string, removeExisting bool) *chttp.Request[containersv1.StartRequest] {
	return chttp.NewRequest(&containersv1.StartRequest{Name: name, RemoveExisting: removeExisting})
}
func newContainerStopReq(name string) *chttp.Request[containersv1.StopRequest] {
	return chttp.NewRequest(&containersv1.StopRequest{Name: name})
}
func newContainerDeleteReq(name string) *chttp.Request[containersv1.DeleteRequest] {
	return chttp.NewRequest(&containersv1.DeleteRequest{Name: name})
}
func newContainerGetReq(name string) *chttp.Request[containersv1.GetRequest] {
	return chttp.NewRequest(&containersv1.GetRequest{Name: name})
}

func (s *ServerSuite) TestContainerCreate() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	name := testutil.UniqueName(s.T())
	defer s.cleanupContainer(name)

	resp, err := s.containers.Create(ctx, newContainerCreateReq(name))
	s.Require().NoError(err)

	c := resp.Msg.Container
	s.NotEmpty(c.Id)
	s.Equal(name, c.Name)
	s.Equal(containersv1.ContainerStatus_CONTAINER_STATUS_STOPPED, c.Status)
}

func (s *ServerSuite) TestContainerCreateDuplicate() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	name := testutil.UniqueName(s.T())
	defer s.cleanupContainer(name)

	_, err := s.containers.Create(ctx, newContainerCreateReq(name))
	s.Require().NoError(err)

	_, err = s.containers.Create(ctx, newContainerCreateReq(name))
	s.Require().Error(err)
	s.Equal(chttp.CodeAlreadyExists, chttp.CodeOf(err))
}

func (s *ServerSuite) TestContainerCreateInvalidName() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := s.containers.Create(ctx, newContainerCreateReq("INVALID_NAME!"))
	s.Require().Error(err)
	s.Equal(chttp.CodeInvalidArgument, chttp.CodeOf(err))
}

// TestContainerLifecycle tests start → stop → restart (new DockerID) → delete
// in a single devcontainer to avoid repeated ~14s startup cost.
func (s *ServerSuite) TestContainerLifecycle() {
	name := testutil.UniqueName(s.T())
	defer s.cleanupContainer(name)

	s.mustStartContainer(name)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Verify running.
	got, err := s.containers.Get(ctx, newContainerGetReq(name))
	s.Require().NoError(err)
	s.NotEmpty(got.Msg.Container.DockerId)
	s.Equal(containersv1.ContainerStatus_CONTAINER_STATUS_RUNNING, got.Msg.Container.Status)
	firstDockerID := got.Msg.Container.DockerId

	// Stop.
	stopped, err := s.containers.Stop(ctx, newContainerStopReq(name))
	s.Require().NoError(err)
	s.Equal(containersv1.ContainerStatus_CONTAINER_STATUS_STOPPED, stopped.Msg.Container.Status)

	// Restart — should get a new DockerID.
	_, err = s.containers.Start(ctx, newContainerStartReq(name, true))
	s.Require().NoError(err)

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer waitCancel()
	for {
		got, err := s.containers.Get(waitCtx, newContainerGetReq(name))
		s.Require().NoError(err)
		if got.Msg.Container.Status == containersv1.ContainerStatus_CONTAINER_STATUS_RUNNING {
			s.NotEqual(firstDockerID, got.Msg.Container.DockerId, "expected different DockerID after restart")
			break
		}
		select {
		case <-waitCtx.Done():
			s.Fail("timeout waiting for restart")
			return
		case <-time.After(2 * time.Second):
		}
	}

	// Delete.
	_, err = s.containers.Delete(waitCtx, newContainerDeleteReq(name))
	s.Require().NoError(err)

	_, err = s.containers.Get(waitCtx, newContainerGetReq(name))
	s.Require().Error(err)
	s.Equal(chttp.CodeNotFound, chttp.CodeOf(err))
}

func (s *ServerSuite) TestContainerList() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	name := testutil.UniqueName(s.T())
	defer s.cleanupContainer(name)

	_, err := s.containers.Create(ctx, newContainerCreateReq(name))
	s.Require().NoError(err)

	list, err := s.containers.List(ctx, chttp.NewRequest(&containersv1.ListRequest{}))
	s.Require().NoError(err)

	found := false
	for _, c := range list.Msg.Containers {
		if c.Name == name {
			found = true
			break
		}
	}
	s.True(found, "expected container %q in list", name)
}

func (s *ServerSuite) TestContainerLogs() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := s.containers.ListLogs(ctx, chttp.NewRequest(&containersv1.ListLogsRequest{
		Name: s.sharedInstance,
	}))
	s.Require().NoError(err)
	s.Require().NotEmpty(resp.Msg.Files)

	f := resp.Msg.Files[0]
	s.NotEmpty(f.Id)
	s.NotEmpty(f.Source)
}

func (s *ServerSuite) TestContainerAuth() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	noAuth := containersv1connect.NewContainerServiceClient(testutil.H2CClient(), s.baseURL)
	_, err := noAuth.List(ctx, chttp.NewRequest(&containersv1.ListRequest{}))
	s.Error(err)
}
