// Package terminal proxies terminal operations to the daemon via gRPC.
package terminal

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"go.uber.org/zap"

	terminalspb "github.com/exa-pub/norn/internal/gen/norn/daemon/terminals/v1"
	"github.com/exa-pub/norn/internal/server/service/daemonconn"
)

type Service interface {
	Create(ctx context.Context, instanceName, name string) (*TerminalSession, error)
	Get(ctx context.Context, instanceName, id string) (*TerminalSession, error)
	List(ctx context.Context, instanceName string) []*TerminalSession
	Close(ctx context.Context, instanceName, id string) error
	Rename(ctx context.Context, instanceName, id, name string) (*TerminalSession, error)
}

type service struct {
	pool *daemonconn.Pool
}

func NewService(pool *daemonconn.Pool) Service {
	return &service{pool: pool}
}

func (s *service) Create(ctx context.Context, instanceName, name string) (*TerminalSession, error) {
	conn, err := s.pool.Get(instanceName)
	if err != nil {
		return nil, fmt.Errorf("daemon connection: %w", err)
	}
	resp, err := conn.Terminals.Create(ctx, connect.NewRequest(&terminalspb.CreateRequest{
		Name: name,
	}))
	if err != nil {
		return nil, err
	}
	t := resp.Msg.Terminal
	return &TerminalSession{
		ID:           t.Id,
		InstanceName: instanceName,
		Name:         t.Name,
		TTYID:        t.TtyId,
	}, nil
}

func (s *service) Get(ctx context.Context, instanceName, id string) (*TerminalSession, error) {
	conn, err := s.pool.Get(instanceName)
	if err != nil {
		return nil, fmt.Errorf("daemon connection: %w", err)
	}
	// Daemon terminals proto doesn't have a Get RPC — use List and filter.
	resp, err := conn.Terminals.List(ctx, connect.NewRequest(&terminalspb.ListRequest{}))
	if err != nil {
		return nil, err
	}
	for _, t := range resp.Msg.Terminals {
		if t.Id == id {
			return &TerminalSession{
				ID:           t.Id,
				InstanceName: instanceName,
				Name:         t.Name,
				TTYID:        t.TtyId,
			}, nil
		}
	}
	return nil, fmt.Errorf("terminal %q: %w", id, ErrNotFound)
}

func (s *service) List(ctx context.Context, instanceName string) []*TerminalSession {
	conn, err := s.pool.Get(instanceName)
	if err != nil {
		zap.L().Warn("terminal list: daemon connection failed", zap.String("instance", instanceName), zap.Error(err))
		return nil
	}
	resp, err := conn.Terminals.List(ctx, connect.NewRequest(&terminalspb.ListRequest{}))
	if err != nil {
		zap.L().Warn("terminal list: rpc failed", zap.String("instance", instanceName), zap.Error(err))
		return nil
	}
	result := make([]*TerminalSession, 0, len(resp.Msg.Terminals))
	for _, t := range resp.Msg.Terminals {
		result = append(result, &TerminalSession{
			ID:           t.Id,
			InstanceName: instanceName,
			Name:         t.Name,
			TTYID:        t.TtyId,
		})
	}
	return result
}

func (s *service) Close(ctx context.Context, instanceName, id string) error {
	conn, err := s.pool.Get(instanceName)
	if err != nil {
		return fmt.Errorf("daemon connection: %w", err)
	}
	_, err = conn.Terminals.Delete(ctx, connect.NewRequest(&terminalspb.DeleteRequest{
		TerminalId: id,
	}))
	return err
}

func (s *service) Rename(ctx context.Context, instanceName, id, name string) (*TerminalSession, error) {
	conn, err := s.pool.Get(instanceName)
	if err != nil {
		return nil, fmt.Errorf("daemon connection: %w", err)
	}
	resp, err := conn.Terminals.Rename(ctx, connect.NewRequest(&terminalspb.RenameRequest{
		TerminalId: id,
		Name:       name,
	}))
	if err != nil {
		return nil, err
	}
	t := resp.Msg.Terminal
	return &TerminalSession{
		ID:           t.Id,
		InstanceName: instanceName,
		Name:         t.Name,
		TTYID:        t.TtyId,
	}, nil
}
