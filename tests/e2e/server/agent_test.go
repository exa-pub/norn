package server

import (
	"context"
	"time"

	chttp "connectrpc.com/connect"

	agentsv1 "github.com/exa-pub/norn/internal/gen/norn/server/agents/v1"
	"github.com/exa-pub/norn/tests/e2e/testutil"
)

func (s *ServerSuite) TestAgentCreate() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := s.agents.Create(ctx, chttp.NewRequest(&agentsv1.CreateRequest{
		InstanceName: s.sharedInstance, Name: "my-agent",
	}))
	s.Require().NoError(err)

	sess := resp.Msg.Session
	s.NotEmpty(sess.Id)
	s.Equal("my-agent", sess.Name)
	s.True(sess.Running, "expected Running=true after create")
	s.NotEmpty(sess.TtyId)

	s.agents.Delete(ctx, chttp.NewRequest(&agentsv1.DeleteRequest{
		InstanceName: s.sharedInstance, SessionId: sess.Id,
	}))
}

func (s *ServerSuite) TestAgentStopAndResume() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	created, err := s.agents.Create(ctx, chttp.NewRequest(&agentsv1.CreateRequest{
		InstanceName: s.sharedInstance, Name: "stop-resume",
	}))
	s.Require().NoError(err)
	sessID := created.Msg.Session.Id
	defer s.agents.Delete(ctx, chttp.NewRequest(&agentsv1.DeleteRequest{
		InstanceName: s.sharedInstance, SessionId: sessID,
	}))

	stopped, err := s.agents.Stop(ctx, chttp.NewRequest(&agentsv1.StopRequest{
		InstanceName: s.sharedInstance, SessionId: sessID,
	}))
	s.Require().NoError(err)
	s.False(stopped.Msg.Session.Running)

	resumed, err := s.agents.Start(ctx, chttp.NewRequest(&agentsv1.StartRequest{
		InstanceName: s.sharedInstance, SessionId: sessID,
	}))
	s.Require().NoError(err)
	s.True(resumed.Msg.Session.Running)
}

func (s *ServerSuite) TestAgentRename() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	created, err := s.agents.Create(ctx, chttp.NewRequest(&agentsv1.CreateRequest{
		InstanceName: s.sharedInstance, Name: "orig-name",
	}))
	s.Require().NoError(err)
	defer s.agents.Delete(ctx, chttp.NewRequest(&agentsv1.DeleteRequest{
		InstanceName: s.sharedInstance, SessionId: created.Msg.Session.Id,
	}))

	renamed, err := s.agents.Rename(ctx, chttp.NewRequest(&agentsv1.RenameRequest{
		InstanceName: s.sharedInstance, SessionId: created.Msg.Session.Id, Name: "new-name",
	}))
	s.Require().NoError(err)
	s.Equal("new-name", renamed.Msg.Session.Name)
}

func (s *ServerSuite) TestAgentDelete() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	created, err := s.agents.Create(ctx, chttp.NewRequest(&agentsv1.CreateRequest{
		InstanceName: s.sharedInstance, Name: "del-agent",
	}))
	s.Require().NoError(err)

	s.agents.Stop(ctx, chttp.NewRequest(&agentsv1.StopRequest{
		InstanceName: s.sharedInstance, SessionId: created.Msg.Session.Id,
	}))

	_, err = s.agents.Delete(ctx, chttp.NewRequest(&agentsv1.DeleteRequest{
		InstanceName: s.sharedInstance, SessionId: created.Msg.Session.Id,
	}))
	s.Require().NoError(err)

	_, err = s.agents.Get(ctx, chttp.NewRequest(&agentsv1.GetRequest{
		InstanceName: s.sharedInstance, SessionId: created.Msg.Session.Id,
	}))
	s.Error(err)
}

func (s *ServerSuite) TestAgentDeleteWhileRunning() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	created, err := s.agents.Create(ctx, chttp.NewRequest(&agentsv1.CreateRequest{
		InstanceName: s.sharedInstance, Name: "running-agent",
	}))
	s.Require().NoError(err)
	s.True(created.Msg.Session.Running)

	_, err = s.agents.Delete(ctx, chttp.NewRequest(&agentsv1.DeleteRequest{
		InstanceName: s.sharedInstance, SessionId: created.Msg.Session.Id,
	}))
	s.Require().NoError(err)

	_, err = s.agents.Get(ctx, chttp.NewRequest(&agentsv1.GetRequest{
		InstanceName: s.sharedInstance, SessionId: created.Msg.Session.Id,
	}))
	s.Error(err)
}

func (s *ServerSuite) TestAgentList() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c1, err := s.agents.Create(ctx, chttp.NewRequest(&agentsv1.CreateRequest{
		InstanceName: s.sharedInstance, Name: "list-a1",
	}))
	s.Require().NoError(err)
	defer s.agents.Delete(ctx, chttp.NewRequest(&agentsv1.DeleteRequest{
		InstanceName: s.sharedInstance, SessionId: c1.Msg.Session.Id,
	}))

	c2, err := s.agents.Create(ctx, chttp.NewRequest(&agentsv1.CreateRequest{
		InstanceName: s.sharedInstance, Name: "list-a2",
	}))
	s.Require().NoError(err)
	defer s.agents.Delete(ctx, chttp.NewRequest(&agentsv1.DeleteRequest{
		InstanceName: s.sharedInstance, SessionId: c2.Msg.Session.Id,
	}))

	list, err := s.agents.List(ctx, chttp.NewRequest(&agentsv1.ListRequest{
		InstanceName: s.sharedInstance,
	}))
	s.Require().NoError(err)

	found := 0
	for _, sess := range list.Msg.Sessions {
		if sess.Id == c1.Msg.Session.Id || sess.Id == c2.Msg.Session.Id {
			found++
		}
	}
	s.Equal(2, found)
}

func (s *ServerSuite) TestAgentCreateNoContainer() {
	name := testutil.UniqueName(s.T())
	defer s.cleanupContainer(name)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := s.containers.Create(ctx, newContainerCreateReq(name))
	s.Require().NoError(err)

	_, err = s.agents.Create(ctx, chttp.NewRequest(&agentsv1.CreateRequest{
		InstanceName: name, Name: "no-container-agent",
	}))
	s.Error(err, "expected error when creating agent without running container")
}
