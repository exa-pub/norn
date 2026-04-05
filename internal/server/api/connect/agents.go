package connect

import (
	"context"

	chttp "connectrpc.com/connect"

	agentsv1 "github.com/exa-pub/norn/internal/gen/norn/server/agents/v1"
	"github.com/exa-pub/norn/internal/gen/norn/server/agents/v1/agentsv1connect"
	"github.com/exa-pub/norn/internal/server/service/agent"
)

var _ agentsv1connect.AgentServiceHandler = (*AgentHandler)(nil)

type AgentHandler struct {
	svc agent.Service
}

func NewAgentHandler(svc agent.Service) *AgentHandler {
	return &AgentHandler{svc: svc}
}

func (h *AgentHandler) Create(ctx context.Context, req *chttp.Request[agentsv1.CreateRequest]) (*chttp.Response[agentsv1.CreateResponse], error) {
	sess, err := h.svc.CreateSession(ctx, req.Msg.InstanceName, req.Msg.Name)
	if err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&agentsv1.CreateResponse{Session: agentToProto(sess)}), nil
}

func (h *AgentHandler) Start(ctx context.Context, req *chttp.Request[agentsv1.StartRequest]) (*chttp.Response[agentsv1.StartResponse], error) {
	sess, err := h.svc.Launch(ctx, req.Msg.InstanceName, req.Msg.SessionId, req.Msg.Prompt)
	if err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&agentsv1.StartResponse{Session: agentToProto(sess)}), nil
}

func (h *AgentHandler) Stop(ctx context.Context, req *chttp.Request[agentsv1.StopRequest]) (*chttp.Response[agentsv1.StopResponse], error) {
	sess, err := h.svc.Stop(ctx, req.Msg.InstanceName, req.Msg.SessionId)
	if err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&agentsv1.StopResponse{Session: agentToProto(sess)}), nil
}

func (h *AgentHandler) Delete(ctx context.Context, req *chttp.Request[agentsv1.DeleteRequest]) (*chttp.Response[agentsv1.DeleteResponse], error) {
	if err := h.svc.DeleteSession(ctx, req.Msg.InstanceName, req.Msg.SessionId); err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&agentsv1.DeleteResponse{}), nil
}

func (h *AgentHandler) Rename(ctx context.Context, req *chttp.Request[agentsv1.RenameRequest]) (*chttp.Response[agentsv1.RenameResponse], error) {
	sess, err := h.svc.UpdateSessionName(ctx, req.Msg.InstanceName, req.Msg.SessionId, req.Msg.Name)
	if err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&agentsv1.RenameResponse{Session: agentToProto(sess)}), nil
}

func (h *AgentHandler) Get(ctx context.Context, req *chttp.Request[agentsv1.GetRequest]) (*chttp.Response[agentsv1.GetResponse], error) {
	sess, err := h.svc.GetSession(ctx, req.Msg.InstanceName, req.Msg.SessionId)
	if err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&agentsv1.GetResponse{Session: agentToProto(sess)}), nil
}

func (h *AgentHandler) List(ctx context.Context, req *chttp.Request[agentsv1.ListRequest]) (*chttp.Response[agentsv1.ListResponse], error) {
	list, err := h.svc.ListSessions(ctx, req.Msg.InstanceName)
	if err != nil {
		return nil, toConnectError(err)
	}
	resp := &agentsv1.ListResponse{}
	for _, sess := range list {
		resp.Sessions = append(resp.Sessions, agentToProto(sess))
	}
	return chttp.NewResponse(resp), nil
}

func agentToProto(sess *agent.AgentSession) *agentsv1.AgentSession {
	return &agentsv1.AgentSession{
		Id:      sess.ID,
		Name:    sess.Name,
		Running: sess.Running,
		TtyId:   sess.TTYID,
	}
}
