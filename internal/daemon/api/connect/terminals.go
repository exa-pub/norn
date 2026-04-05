package connect

import (
	"context"

	chttp "connectrpc.com/connect"

	terminalspb "github.com/exa-pub/norn/internal/gen/norn/daemon/terminals/v1"
	"github.com/exa-pub/norn/internal/gen/norn/daemon/terminals/v1/terminalsv1connect"

	"github.com/exa-pub/norn/internal/daemon/service/terminal"
)

var _ terminalsv1connect.TerminalServiceHandler = (*TerminalHandler)(nil)

type TerminalHandler struct {
	svc *terminal.Service
}

func NewTerminalHandler(svc *terminal.Service) *TerminalHandler {
	return &TerminalHandler{svc: svc}
}

func (h *TerminalHandler) Create(ctx context.Context, req *chttp.Request[terminalspb.CreateRequest]) (*chttp.Response[terminalspb.CreateResponse], error) {
	t, err := h.svc.Create(req.Msg.Name, req.Msg.Cmd)
	if err != nil {
		return nil, chttp.NewError(chttp.CodeInternal, err)
	}
	return chttp.NewResponse(&terminalspb.CreateResponse{Terminal: termToProto(t)}), nil
}

func (h *TerminalHandler) Delete(ctx context.Context, req *chttp.Request[terminalspb.DeleteRequest]) (*chttp.Response[terminalspb.DeleteResponse], error) {
	if err := h.svc.Delete(req.Msg.TerminalId); err != nil {
		return nil, chttp.NewError(chttp.CodeInternal, err)
	}
	return chttp.NewResponse(&terminalspb.DeleteResponse{}), nil
}

func (h *TerminalHandler) Rename(ctx context.Context, req *chttp.Request[terminalspb.RenameRequest]) (*chttp.Response[terminalspb.RenameResponse], error) {
	t, err := h.svc.Rename(req.Msg.TerminalId, req.Msg.Name)
	if err != nil {
		return nil, chttp.NewError(chttp.CodeInternal, err)
	}
	return chttp.NewResponse(&terminalspb.RenameResponse{Terminal: termToProto(t)}), nil
}

func (h *TerminalHandler) List(ctx context.Context, req *chttp.Request[terminalspb.ListRequest]) (*chttp.Response[terminalspb.ListResponse], error) {
	terms := h.svc.List()
	resp := &terminalspb.ListResponse{}
	for _, t := range terms {
		resp.Terminals = append(resp.Terminals, termToProto(t))
	}
	return chttp.NewResponse(resp), nil
}

func termToProto(t *terminal.Terminal) *terminalspb.Terminal {
	return &terminalspb.Terminal{
		Id:    t.ID,
		Name:  t.Name,
		TtyId: t.TTYID,
	}
}
