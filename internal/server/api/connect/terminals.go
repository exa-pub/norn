package connect

import (
	"context"

	chttp "connectrpc.com/connect"

	terminalsv1 "github.com/exa-pub/norn/internal/gen/norn/server/terminals/v1"
	"github.com/exa-pub/norn/internal/gen/norn/server/terminals/v1/terminalsv1connect"
	"github.com/exa-pub/norn/internal/server/service/terminal"
)

var _ terminalsv1connect.TerminalServiceHandler = (*TerminalHandler)(nil)

type TerminalHandler struct {
	svc terminal.Service
}

func NewTerminalHandler(svc terminal.Service) *TerminalHandler {
	return &TerminalHandler{svc: svc}
}

func (h *TerminalHandler) Create(ctx context.Context, req *chttp.Request[terminalsv1.CreateRequest]) (*chttp.Response[terminalsv1.CreateResponse], error) {
	t, err := h.svc.Create(ctx, req.Msg.InstanceName, req.Msg.Name)
	if err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&terminalsv1.CreateResponse{Terminal: terminalToProto(t)}), nil
}

func (h *TerminalHandler) Delete(ctx context.Context, req *chttp.Request[terminalsv1.DeleteRequest]) (*chttp.Response[terminalsv1.DeleteResponse], error) {
	if err := h.svc.Close(ctx, req.Msg.InstanceName, req.Msg.Id); err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&terminalsv1.DeleteResponse{}), nil
}

func (h *TerminalHandler) Rename(ctx context.Context, req *chttp.Request[terminalsv1.RenameRequest]) (*chttp.Response[terminalsv1.RenameResponse], error) {
	t, err := h.svc.Rename(ctx, req.Msg.InstanceName, req.Msg.Id, req.Msg.Name)
	if err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&terminalsv1.RenameResponse{Terminal: terminalToProto(t)}), nil
}

func (h *TerminalHandler) Get(ctx context.Context, req *chttp.Request[terminalsv1.GetRequest]) (*chttp.Response[terminalsv1.GetResponse], error) {
	t, err := h.svc.Get(ctx, req.Msg.InstanceName, req.Msg.Id)
	if err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&terminalsv1.GetResponse{Terminal: terminalToProto(t)}), nil
}

func (h *TerminalHandler) List(ctx context.Context, req *chttp.Request[terminalsv1.ListRequest]) (*chttp.Response[terminalsv1.ListResponse], error) {
	list := h.svc.List(ctx, req.Msg.InstanceName)
	resp := &terminalsv1.ListResponse{}
	for _, t := range list {
		resp.Terminals = append(resp.Terminals, terminalToProto(t))
	}
	return chttp.NewResponse(resp), nil
}

func terminalToProto(t *terminal.TerminalSession) *terminalsv1.Terminal {
	return &terminalsv1.Terminal{
		Id:           t.ID,
		InstanceName: t.InstanceName,
		Name:         t.Name,
		TtyId:        t.TTYID,
	}
}
