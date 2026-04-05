package server

import (
	"context"
	"time"

	chttp "connectrpc.com/connect"

	terminalsv1 "github.com/exa-pub/norn/internal/gen/norn/server/terminals/v1"
)

func (s *ServerSuite) TestTerminalCreate() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := s.terminals.Create(ctx, chttp.NewRequest(&terminalsv1.CreateRequest{
		InstanceName: s.sharedInstance, Name: "test-terminal",
	}))
	s.Require().NoError(err)

	term := resp.Msg.Terminal
	s.NotEmpty(term.Id)
	s.NotEmpty(term.TtyId)
	s.Equal(s.sharedInstance, term.InstanceName)

	s.terminals.Delete(ctx, chttp.NewRequest(&terminalsv1.DeleteRequest{
		InstanceName: s.sharedInstance, Id: term.Id,
	}))
}

func (s *ServerSuite) TestTerminalDelete() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	created, err := s.terminals.Create(ctx, chttp.NewRequest(&terminalsv1.CreateRequest{
		InstanceName: s.sharedInstance, Name: "del-terminal",
	}))
	s.Require().NoError(err)

	_, err = s.terminals.Delete(ctx, chttp.NewRequest(&terminalsv1.DeleteRequest{
		InstanceName: s.sharedInstance, Id: created.Msg.Terminal.Id,
	}))
	s.Require().NoError(err)

	_, err = s.terminals.Get(ctx, chttp.NewRequest(&terminalsv1.GetRequest{
		InstanceName: s.sharedInstance, Id: created.Msg.Terminal.Id,
	}))
	s.Require().Error(err)
	s.Equal(chttp.CodeNotFound, chttp.CodeOf(err))
}

func (s *ServerSuite) TestTerminalRename() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	created, err := s.terminals.Create(ctx, chttp.NewRequest(&terminalsv1.CreateRequest{
		InstanceName: s.sharedInstance, Name: "rename-orig",
	}))
	s.Require().NoError(err)
	defer s.terminals.Delete(ctx, chttp.NewRequest(&terminalsv1.DeleteRequest{
		InstanceName: s.sharedInstance, Id: created.Msg.Terminal.Id,
	}))

	renamed, err := s.terminals.Rename(ctx, chttp.NewRequest(&terminalsv1.RenameRequest{
		InstanceName: s.sharedInstance, Id: created.Msg.Terminal.Id, Name: "rename-new",
	}))
	s.Require().NoError(err)
	s.Equal("rename-new", renamed.Msg.Terminal.Name)
}

func (s *ServerSuite) TestTerminalList() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c1, err := s.terminals.Create(ctx, chttp.NewRequest(&terminalsv1.CreateRequest{
		InstanceName: s.sharedInstance, Name: "list-t1",
	}))
	s.Require().NoError(err)
	defer s.terminals.Delete(ctx, chttp.NewRequest(&terminalsv1.DeleteRequest{
		InstanceName: s.sharedInstance, Id: c1.Msg.Terminal.Id,
	}))

	c2, err := s.terminals.Create(ctx, chttp.NewRequest(&terminalsv1.CreateRequest{
		InstanceName: s.sharedInstance, Name: "list-t2",
	}))
	s.Require().NoError(err)
	defer s.terminals.Delete(ctx, chttp.NewRequest(&terminalsv1.DeleteRequest{
		InstanceName: s.sharedInstance, Id: c2.Msg.Terminal.Id,
	}))

	list, err := s.terminals.List(ctx, chttp.NewRequest(&terminalsv1.ListRequest{
		InstanceName: s.sharedInstance,
	}))
	s.Require().NoError(err)

	found := 0
	for _, term := range list.Msg.Terminals {
		if term.Id == c1.Msg.Terminal.Id || term.Id == c2.Msg.Terminal.Id {
			found++
		}
	}
	s.Equal(2, found)
}
