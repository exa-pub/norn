package connect

import (
	"context"

	chttp "connectrpc.com/connect"

	"github.com/exa-pub/norn/internal/entity"
	agentsv1 "github.com/exa-pub/norn/internal/gen/norn/agents/v1"
	"github.com/exa-pub/norn/internal/gen/norn/agents/v1/agentsv1connect"
	"github.com/exa-pub/norn/internal/service/agent"
)

var _ agentsv1connect.AgentServiceHandler = (*AgentHandler)(nil)

type AgentHandler struct {
	svc agent.Service
}

func NewAgentHandler(svc agent.Service) *AgentHandler {
	return &AgentHandler{svc: svc}
}

func (h *AgentHandler) CreateAgentSession(ctx context.Context, req *chttp.Request[agentsv1.CreateAgentSessionRequest]) (*chttp.Response[agentsv1.CreateAgentSessionResponse], error) {
	sess, err := h.svc.CreateSession(ctx, req.Msg.InstanceName, req.Msg.Name)
	if err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&agentsv1.CreateAgentSessionResponse{Session: agentToProto(sess)}), nil
}

func (h *AgentHandler) GetAgentSession(ctx context.Context, req *chttp.Request[agentsv1.GetAgentSessionRequest]) (*chttp.Response[agentsv1.GetAgentSessionResponse], error) {
	sess, err := h.svc.GetSession(ctx, req.Msg.InstanceName, req.Msg.SessionId)
	if err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&agentsv1.GetAgentSessionResponse{Session: agentToProto(sess)}), nil
}

func (h *AgentHandler) ListAgentSessions(ctx context.Context, req *chttp.Request[agentsv1.ListAgentSessionsRequest]) (*chttp.Response[agentsv1.ListAgentSessionsResponse], error) {
	list, err := h.svc.ListSessions(ctx, req.Msg.InstanceName)
	if err != nil {
		return nil, toConnectError(err)
	}
	resp := &agentsv1.ListAgentSessionsResponse{}
	for _, sess := range list {
		resp.Sessions = append(resp.Sessions, agentToProto(sess))
	}
	return chttp.NewResponse(resp), nil
}

func (h *AgentHandler) DeleteAgentSession(ctx context.Context, req *chttp.Request[agentsv1.DeleteAgentSessionRequest]) (*chttp.Response[agentsv1.DeleteAgentSessionResponse], error) {
	if err := h.svc.DeleteSession(ctx, req.Msg.InstanceName, req.Msg.SessionId); err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&agentsv1.DeleteAgentSessionResponse{}), nil
}

func (h *AgentHandler) UpdateAgentSessionName(ctx context.Context, req *chttp.Request[agentsv1.UpdateAgentSessionNameRequest]) (*chttp.Response[agentsv1.UpdateAgentSessionNameResponse], error) {
	sess, err := h.svc.UpdateSessionName(ctx, req.Msg.InstanceName, req.Msg.SessionId, req.Msg.Name)
	if err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&agentsv1.UpdateAgentSessionNameResponse{Session: agentToProto(sess)}), nil
}

func (h *AgentHandler) LaunchAgent(ctx context.Context, req *chttp.Request[agentsv1.LaunchAgentRequest]) (*chttp.Response[agentsv1.LaunchAgentResponse], error) {
	sess, err := h.svc.Launch(ctx, req.Msg.InstanceName, req.Msg.SessionId, req.Msg.Prompt)
	if err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&agentsv1.LaunchAgentResponse{Session: agentToProto(sess)}), nil
}

func (h *AgentHandler) StopAgent(ctx context.Context, req *chttp.Request[agentsv1.StopAgentRequest]) (*chttp.Response[agentsv1.StopAgentResponse], error) {
	sess, err := h.svc.Stop(ctx, req.Msg.InstanceName, req.Msg.SessionId)
	if err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&agentsv1.StopAgentResponse{Session: agentToProto(sess)}), nil
}

func agentToProto(sess *entity.AgentSession) *agentsv1.AgentSession {
	return &agentsv1.AgentSession{
		Id:      sess.ID,
		Name:    sess.Name,
		Running: sess.Running,
		TtyId:   sess.TTYID,
	}
}

