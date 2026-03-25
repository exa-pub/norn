package connect

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/exa-pub/norn/internal/container"
	"github.com/exa-pub/norn/internal/entity"
	containersv1 "github.com/exa-pub/norn/internal/gen/norn/containers/v1"
	"github.com/exa-pub/norn/internal/gen/norn/containers/v1/containersv1connect"
)

// Ensure ContainerHandler implements the generated interface at compile time.
var _ containersv1connect.ContainerServiceHandler = (*ContainerHandler)(nil)

type ContainerHandler struct {
	mgr container.Manager
}

func NewContainerHandler(mgr container.Manager) *ContainerHandler {
	return &ContainerHandler{mgr: mgr}
}

func (h *ContainerHandler) CreateContainer(
	ctx context.Context,
	req *connect.Request[containersv1.CreateContainerRequest],
) (*connect.Response[containersv1.CreateContainerResponse], error) {
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}

	c, err := h.mgr.Create(ctx, req.Msg.Name)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&containersv1.CreateContainerResponse{
		Container: toProto(c),
	}), nil
}

func (h *ContainerHandler) GetContainer(
	ctx context.Context,
	req *connect.Request[containersv1.GetContainerRequest],
) (*connect.Response[containersv1.GetContainerResponse], error) {
	c, err := h.mgr.Get(ctx, req.Msg.Name)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&containersv1.GetContainerResponse{
		Container: toProto(c),
	}), nil
}

func (h *ContainerHandler) ListContainers(
	ctx context.Context,
	_ *connect.Request[containersv1.ListContainersRequest],
) (*connect.Response[containersv1.ListContainersResponse], error) {
	containers, err := h.mgr.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &containersv1.ListContainersResponse{}
	for _, c := range containers {
		resp.Containers = append(resp.Containers, toProto(c))
	}

	return connect.NewResponse(resp), nil
}

func (h *ContainerHandler) StartContainer(
	ctx context.Context,
	req *connect.Request[containersv1.StartContainerRequest],
) (*connect.Response[containersv1.StartContainerResponse], error) {
	c, err := h.mgr.Start(ctx, req.Msg.Name, container.StartOptions{})
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&containersv1.StartContainerResponse{
		Container: toProto(c),
	}), nil
}

func (h *ContainerHandler) StopContainer(
	ctx context.Context,
	req *connect.Request[containersv1.StopContainerRequest],
) (*connect.Response[containersv1.StopContainerResponse], error) {
	c, err := h.mgr.Stop(ctx, req.Msg.Name)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&containersv1.StopContainerResponse{
		Container: toProto(c),
	}), nil
}

func (h *ContainerHandler) DeleteContainer(
	ctx context.Context,
	req *connect.Request[containersv1.DeleteContainerRequest],
) (*connect.Response[containersv1.DeleteContainerResponse], error) {
	if err := h.mgr.Delete(ctx, req.Msg.Name); err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&containersv1.DeleteContainerResponse{}), nil
}

func (h *ContainerHandler) StreamLogs(
	ctx context.Context,
	req *connect.Request[containersv1.StreamLogsRequest],
	stream *connect.ServerStream[containersv1.StreamLogsResponse],
) error {
	bus, ok := h.mgr.LogBusFor(req.Msg.Name)
	if !ok {
		// Container is not currently starting — no live logs.
		return nil
	}

	ch := bus.Subscribe(ctx)
	for entry := range ch {
		err := stream.Send(&containersv1.StreamLogsResponse{
			Name:      req.Msg.Name,
			Line:      entry.Line,
			IsStderr:  entry.IsStderr,
			Timestamp: timestamppb.New(entry.At),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// --- helpers ---

func toProto(c *entity.Container) *containersv1.Container {
	return &containersv1.Container{
		Id:           c.ID,
		Name:         c.Name,
		DockerId:     c.DockerID,
		Status:       toProtoStatus(c.Status),
		ErrorMessage: c.ErrorMessage,
		CreatedAt:    timestamppb.New(c.CreatedAt),
	}
}

func toProtoStatus(s entity.ContainerStatus) containersv1.ContainerStatus {
	switch s {
	case entity.ContainerStatusStarting:
		return containersv1.ContainerStatus_CONTAINER_STATUS_STARTING
	case entity.ContainerStatusRunning:
		return containersv1.ContainerStatus_CONTAINER_STATUS_RUNNING
	case entity.ContainerStatusStopped:
		return containersv1.ContainerStatus_CONTAINER_STATUS_STOPPED
	case entity.ContainerStatusError:
		return containersv1.ContainerStatus_CONTAINER_STATUS_ERROR
	default:
		return containersv1.ContainerStatus_CONTAINER_STATUS_UNSPECIFIED
	}
}

func toConnectError(err error) error {
	switch {
	case errors.Is(err, container.ErrAlreadyExists):
		return connect.NewError(connect.CodeAlreadyExists, err)
	case errors.Is(err, container.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, container.ErrFailedPrecondition):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, container.ErrInvalidName):
		return connect.NewError(connect.CodeInvalidArgument, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}
