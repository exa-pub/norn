// Package agent proxies agent operations to the daemon via gRPC.
package agent

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	agentspb "github.com/exa-pub/norn/internal/gen/norn/daemon/agents/v1"
	"github.com/exa-pub/norn/internal/server/service/daemonconn"
)

type Service interface {
	CreateSession(ctx context.Context, instanceName, name string) (*AgentSession, error)
	GetSession(ctx context.Context, instanceName, sessionID string) (*AgentSession, error)
	ListSessions(ctx context.Context, instanceName string) ([]*AgentSession, error)
	DeleteSession(ctx context.Context, instanceName, sessionID string) error
	UpdateSessionName(ctx context.Context, instanceName, sessionID, name string) (*AgentSession, error)
	Launch(ctx context.Context, instanceName, sessionID, prompt string) (*AgentSession, error)
	Stop(ctx context.Context, instanceName, sessionID string) (*AgentSession, error)
}

type service struct {
	pool *daemonconn.Pool
}

func NewService(pool *daemonconn.Pool) Service {
	return &service{pool: pool}
}

func (s *service) CreateSession(ctx context.Context, instanceName, name string) (*AgentSession, error) {
	conn, err := s.pool.Get(instanceName)
	if err != nil {
		return nil, fmt.Errorf("daemon connection: %w", err)
	}
	resp, err := conn.Agents.Create(ctx, connect.NewRequest(&agentspb.CreateRequest{
		Name: name,
	}))
	if err != nil {
		return nil, err
	}
	return fromProto(resp.Msg.Session), nil
}

func (s *service) GetSession(ctx context.Context, instanceName, sessionID string) (*AgentSession, error) {
	conn, err := s.pool.Get(instanceName)
	if err != nil {
		return nil, fmt.Errorf("daemon connection: %w", err)
	}
	resp, err := conn.Agents.Get(ctx, connect.NewRequest(&agentspb.GetRequest{
		SessionId: sessionID,
	}))
	if err != nil {
		return nil, err
	}
	return fromProto(resp.Msg.Session), nil
}

func (s *service) ListSessions(ctx context.Context, instanceName string) ([]*AgentSession, error) {
	conn, err := s.pool.Get(instanceName)
	if err != nil {
		return nil, fmt.Errorf("daemon connection: %w", err)
	}
	resp, err := conn.Agents.List(ctx, connect.NewRequest(&agentspb.ListRequest{}))
	if err != nil {
		return nil, err
	}
	result := make([]*AgentSession, 0, len(resp.Msg.Sessions))
	for _, sess := range resp.Msg.Sessions {
		result = append(result, fromProto(sess))
	}
	return result, nil
}

func (s *service) DeleteSession(ctx context.Context, instanceName, sessionID string) error {
	conn, err := s.pool.Get(instanceName)
	if err != nil {
		return fmt.Errorf("daemon connection: %w", err)
	}
	_, err = conn.Agents.Delete(ctx, connect.NewRequest(&agentspb.DeleteRequest{
		SessionId: sessionID,
	}))
	return err
}

func (s *service) UpdateSessionName(ctx context.Context, instanceName, sessionID, name string) (*AgentSession, error) {
	conn, err := s.pool.Get(instanceName)
	if err != nil {
		return nil, fmt.Errorf("daemon connection: %w", err)
	}
	resp, err := conn.Agents.Rename(ctx, connect.NewRequest(&agentspb.RenameRequest{
		SessionId: sessionID,
		Name:      name,
	}))
	if err != nil {
		return nil, err
	}
	return fromProto(resp.Msg.Session), nil
}

func (s *service) Launch(ctx context.Context, instanceName, sessionID, prompt string) (*AgentSession, error) {
	conn, err := s.pool.Get(instanceName)
	if err != nil {
		return nil, fmt.Errorf("daemon connection: %w", err)
	}
	resp, err := conn.Agents.Start(ctx, connect.NewRequest(&agentspb.StartRequest{
		SessionId: sessionID,
		Prompt:    prompt,
	}))
	if err != nil {
		return nil, err
	}
	return fromProto(resp.Msg.Session), nil
}

func (s *service) Stop(ctx context.Context, instanceName, sessionID string) (*AgentSession, error) {
	conn, err := s.pool.Get(instanceName)
	if err != nil {
		return nil, fmt.Errorf("daemon connection: %w", err)
	}
	resp, err := conn.Agents.Stop(ctx, connect.NewRequest(&agentspb.StopRequest{
		SessionId: sessionID,
	}))
	if err != nil {
		return nil, err
	}
	return fromProto(resp.Msg.Session), nil
}

func fromProto(s *agentspb.AgentSession) *AgentSession {
	if s == nil {
		return nil
	}
	return &AgentSession{
		ID:      s.Id,
		Name:    s.Name,
		Running: s.Running,
		TTYID:   s.TtyId,
	}
}
