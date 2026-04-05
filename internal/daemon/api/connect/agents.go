package connect

import (
	"context"

	chttp "connectrpc.com/connect"

	agentspb "github.com/exa-pub/norn/internal/gen/norn/daemon/agents/v1"
	"github.com/exa-pub/norn/internal/gen/norn/daemon/agents/v1/agentsv1connect"

	"github.com/exa-pub/norn/internal/daemon/service/agent"
)

var _ agentsv1connect.AgentServiceHandler = (*AgentHandler)(nil)

type AgentHandler struct {
	svc *agent.Service
}

func NewAgentHandler(svc *agent.Service) *AgentHandler {
	return &AgentHandler{svc: svc}
}

func (h *AgentHandler) Create(ctx context.Context, req *chttp.Request[agentspb.CreateRequest]) (*chttp.Response[agentspb.CreateResponse], error) {
	sess, err := h.svc.Create(req.Msg.Name, req.Msg.Prompt)
	if err != nil {
		return nil, chttp.NewError(chttp.CodeInternal, err)
	}
	return chttp.NewResponse(&agentspb.CreateResponse{Session: agentToProto(sess)}), nil
}

func (h *AgentHandler) Start(ctx context.Context, req *chttp.Request[agentspb.StartRequest]) (*chttp.Response[agentspb.StartResponse], error) {
	sess, err := h.svc.Start(req.Msg.SessionId, req.Msg.Prompt)
	if err != nil {
		return nil, chttp.NewError(chttp.CodeInternal, err)
	}
	return chttp.NewResponse(&agentspb.StartResponse{Session: agentToProto(sess)}), nil
}

func (h *AgentHandler) Stop(ctx context.Context, req *chttp.Request[agentspb.StopRequest]) (*chttp.Response[agentspb.StopResponse], error) {
	sess, err := h.svc.Stop(req.Msg.SessionId)
	if err != nil {
		return nil, chttp.NewError(chttp.CodeInternal, err)
	}
	return chttp.NewResponse(&agentspb.StopResponse{Session: agentToProto(sess)}), nil
}

func (h *AgentHandler) Delete(ctx context.Context, req *chttp.Request[agentspb.DeleteRequest]) (*chttp.Response[agentspb.DeleteResponse], error) {
	if err := h.svc.Delete(req.Msg.SessionId); err != nil {
		return nil, chttp.NewError(chttp.CodeInternal, err)
	}
	return chttp.NewResponse(&agentspb.DeleteResponse{}), nil
}

func (h *AgentHandler) Rename(ctx context.Context, req *chttp.Request[agentspb.RenameRequest]) (*chttp.Response[agentspb.RenameResponse], error) {
	sess, err := h.svc.Rename(req.Msg.SessionId, req.Msg.Name)
	if err != nil {
		return nil, chttp.NewError(chttp.CodeInternal, err)
	}
	return chttp.NewResponse(&agentspb.RenameResponse{Session: agentToProto(sess)}), nil
}

func (h *AgentHandler) Get(ctx context.Context, req *chttp.Request[agentspb.GetRequest]) (*chttp.Response[agentspb.GetResponse], error) {
	sess, err := h.svc.Get(req.Msg.SessionId)
	if err != nil {
		return nil, chttp.NewError(chttp.CodeNotFound, err)
	}
	return chttp.NewResponse(&agentspb.GetResponse{Session: agentToProto(sess)}), nil
}

func (h *AgentHandler) List(ctx context.Context, req *chttp.Request[agentspb.ListRequest]) (*chttp.Response[agentspb.ListResponse], error) {
	sessions, err := h.svc.List()
	if err != nil {
		return nil, chttp.NewError(chttp.CodeInternal, err)
	}
	resp := &agentspb.ListResponse{}
	for _, sess := range sessions {
		resp.Sessions = append(resp.Sessions, agentToProto(sess))
	}
	return chttp.NewResponse(resp), nil
}

func agentToProto(sess *agent.AgentSession) *agentspb.AgentSession {
	return &agentspb.AgentSession{
		Id:      sess.ID,
		Name:    sess.Name,
		Running: sess.Running,
		TtyId:   sess.TTYID,
	}
}
