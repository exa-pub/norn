package connect

import (
	"errors"

	chttp "connectrpc.com/connect"

	"github.com/exa-pub/norn/internal/server/service/agent"
	"github.com/exa-pub/norn/internal/server/service/instance"
	"github.com/exa-pub/norn/internal/server/service/terminal"
)

func toConnectError(err error) error {
	switch {
	case errors.Is(err, instance.ErrAlreadyExists):
		return chttp.NewError(chttp.CodeAlreadyExists, err)
	case errors.Is(err, instance.ErrInvalidName):
		return chttp.NewError(chttp.CodeInvalidArgument, err)
	case errors.Is(err, instance.ErrNotFound),
		errors.Is(err, agent.ErrNotFound),
		errors.Is(err, terminal.ErrNotFound):
		return chttp.NewError(chttp.CodeNotFound, err)
	case errors.Is(err, instance.ErrFailedPrecondition),
		errors.Is(err, agent.ErrFailedPrecondition):
		return chttp.NewError(chttp.CodeFailedPrecondition, err)
	default:
		return chttp.NewError(chttp.CodeInternal, err)
	}
}
