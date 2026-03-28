package connect

import (
	"context"

	chttp "connectrpc.com/connect"

	"github.com/exa-pub/norn/internal/entity"
	terminalsv1 "github.com/exa-pub/norn/internal/gen/norn/terminals/v1"
	"github.com/exa-pub/norn/internal/gen/norn/terminals/v1/terminalsv1connect"
	"github.com/exa-pub/norn/internal/service/terminal"
)

var _ terminalsv1connect.TerminalServiceHandler = (*TerminalHandler)(nil)

type TerminalHandler struct {
	svc terminal.Service
}

func NewTerminalHandler(svc terminal.Service) *TerminalHandler {
	return &TerminalHandler{svc: svc}
}

func (h *TerminalHandler) CreateTerminal(ctx context.Context, req *chttp.Request[terminalsv1.CreateTerminalRequest]) (*chttp.Response[terminalsv1.CreateTerminalResponse], error) {
	t, err := h.svc.Create(ctx, req.Msg.InstanceName, req.Msg.Name)
	if err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&terminalsv1.CreateTerminalResponse{Terminal: terminalToProto(t)}), nil
}

func (h *TerminalHandler) GetTerminal(ctx context.Context, req *chttp.Request[terminalsv1.GetTerminalRequest]) (*chttp.Response[terminalsv1.GetTerminalResponse], error) {
	t, err := h.svc.Get(req.Msg.Id)
	if err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&terminalsv1.GetTerminalResponse{Terminal: terminalToProto(t)}), nil
}

func (h *TerminalHandler) ListTerminals(ctx context.Context, req *chttp.Request[terminalsv1.ListTerminalsRequest]) (*chttp.Response[terminalsv1.ListTerminalsResponse], error) {
	list := h.svc.List(req.Msg.InstanceName)
	resp := &terminalsv1.ListTerminalsResponse{}
	for _, t := range list {
		resp.Terminals = append(resp.Terminals, terminalToProto(t))
	}
	return chttp.NewResponse(resp), nil
}

func (h *TerminalHandler) CloseTerminal(ctx context.Context, req *chttp.Request[terminalsv1.CloseTerminalRequest]) (*chttp.Response[terminalsv1.CloseTerminalResponse], error) {
	if err := h.svc.Close(req.Msg.Id); err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&terminalsv1.CloseTerminalResponse{}), nil
}

func (h *TerminalHandler) RenameTerminal(ctx context.Context, req *chttp.Request[terminalsv1.RenameTerminalRequest]) (*chttp.Response[terminalsv1.RenameTerminalResponse], error) {
	t, err := h.svc.Rename(req.Msg.Id, req.Msg.Name)
	if err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&terminalsv1.RenameTerminalResponse{Terminal: terminalToProto(t)}), nil
}

func terminalToProto(t *entity.TerminalSession) *terminalsv1.Terminal {
	return &terminalsv1.Terminal{
		Id:           t.ID,
		InstanceName: t.InstanceName,
		Name:         t.Name,
		TtyId:        t.TTYID,
	}
}
