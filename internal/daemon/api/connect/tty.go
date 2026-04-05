package connect

import (
	"context"
	"fmt"

	chttp "connectrpc.com/connect"

	ttypb "github.com/exa-pub/norn/internal/gen/norn/daemon/tty/v1"
	"github.com/exa-pub/norn/internal/gen/norn/daemon/tty/v1/ttyv1connect"

	"github.com/exa-pub/norn/internal/daemon/service/tty"
)

var _ ttyv1connect.TTYServiceHandler = (*TTYHandler)(nil)

type TTYHandler struct {
	mgr tty.Manager
}

func NewTTYHandler(mgr tty.Manager) *TTYHandler {
	return &TTYHandler{mgr: mgr}
}

func (h *TTYHandler) Attach(ctx context.Context, stream *chttp.BidiStream[ttypb.TTYInput, ttypb.TTYOutput]) error {
	first, err := stream.Receive()
	if err != nil {
		return chttp.NewError(chttp.CodeInvalidArgument, fmt.Errorf("expected attach message: %w", err))
	}
	attach := first.GetAttach()
	if attach == nil {
		return chttp.NewError(chttp.CodeInvalidArgument, fmt.Errorf("first message must be TTYInputAttach"))
	}

	a, err := h.mgr.Attach(attach.TtyId)
	if err != nil {
		return chttp.NewError(chttp.CodeNotFound, err)
	}
	defer a.Detach()

	if len(a.Replay) > 0 {
		if err := stream.Send(&ttypb.TTYOutput{
			Payload: &ttypb.TTYOutput_Replay{Replay: &ttypb.TTYOutputReplay{Data: a.Replay}},
		}); err != nil {
			return err
		}
	}

	stop := make(chan struct{})
	defer close(stop)

	done := make(chan error, 1)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := a.Reader.Read(buf)
			if n > 0 {
				select {
				case <-stop:
					return
				default:
				}
				if sendErr := stream.Send(&ttypb.TTYOutput{
					Payload: &ttypb.TTYOutput_Data{Data: &ttypb.TTYOutputData{Data: buf[:n]}},
				}); sendErr != nil {
					done <- sendErr
					return
				}
			}
			if err != nil {
				select {
				case <-stop:
				default:
					_ = stream.Send(&ttypb.TTYOutput{
						Payload: &ttypb.TTYOutput_Closed{Closed: &ttypb.TTYOutputClosed{ExitCode: 0}},
					})
				}
				done <- nil
				return
			}
		}
	}()

	go func() {
		for {
			msg, err := stream.Receive()
			if err != nil {
				return
			}
			switch p := msg.Payload.(type) {
			case *ttypb.TTYInput_Data:
				a.Write(p.Data.Data)
			case *ttypb.TTYInput_Resize:
				_ = a.Resize(uint16(p.Resize.Cols), uint16(p.Resize.Rows))
			}
		}
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return nil
	}
}
