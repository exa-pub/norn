package connect

import (
	"errors"

	chttp "connectrpc.com/connect"

	"github.com/exa-pub/norn/internal/entity"
)

func toConnectError(err error) error {
	switch {
	case errors.Is(err, entity.ErrAlreadyExists):
		return chttp.NewError(chttp.CodeAlreadyExists, err)
	case errors.Is(err, entity.ErrNotFound):
		return chttp.NewError(chttp.CodeNotFound, err)
	case errors.Is(err, entity.ErrFailedPrecondition):
		return chttp.NewError(chttp.CodeFailedPrecondition, err)
	case errors.Is(err, entity.ErrInvalidName):
		return chttp.NewError(chttp.CodeInvalidArgument, err)
	default:
		return chttp.NewError(chttp.CodeInternal, err)
	}
}
