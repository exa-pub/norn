package connect

import (
	"context"

	chttp "connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/exa-pub/norn/internal/entity"
	containersv1 "github.com/exa-pub/norn/internal/gen/norn/containers/v1"
	"github.com/exa-pub/norn/internal/gen/norn/containers/v1/containersv1connect"
	"github.com/exa-pub/norn/internal/service/instance"
)

var _ containersv1connect.ContainerServiceHandler = (*ContainerHandler)(nil)

type ContainerHandler struct {
	svc instance.Service
}

func NewContainerHandler(svc instance.Service) *ContainerHandler {
	return &ContainerHandler{svc: svc}
}

func (h *ContainerHandler) CreateContainer(ctx context.Context, req *chttp.Request[containersv1.CreateContainerRequest]) (*chttp.Response[containersv1.CreateContainerResponse], error) {
	inst, err := h.svc.Create(ctx, req.Msg.Name)
	if err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&containersv1.CreateContainerResponse{Container: instanceToProto(inst)}), nil
}

func (h *ContainerHandler) GetContainer(ctx context.Context, req *chttp.Request[containersv1.GetContainerRequest]) (*chttp.Response[containersv1.GetContainerResponse], error) {
	inst, err := h.svc.Get(ctx, req.Msg.Name)
	if err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&containersv1.GetContainerResponse{Container: instanceToProto(inst)}), nil
}

func (h *ContainerHandler) ListContainers(ctx context.Context, _ *chttp.Request[containersv1.ListContainersRequest]) (*chttp.Response[containersv1.ListContainersResponse], error) {
	list, err := h.svc.List(ctx)
	if err != nil {
		return nil, chttp.NewError(chttp.CodeInternal, err)
	}
	resp := &containersv1.ListContainersResponse{}
	for _, inst := range list {
		resp.Containers = append(resp.Containers, instanceToProto(inst))
	}
	return chttp.NewResponse(resp), nil
}

func (h *ContainerHandler) StartContainer(ctx context.Context, req *chttp.Request[containersv1.StartContainerRequest]) (*chttp.Response[containersv1.StartContainerResponse], error) {
	inst, err := h.svc.Start(ctx, req.Msg.Name, instance.StartOptions{
		RemoveExisting: req.Msg.RemoveExisting,
	})
	if err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&containersv1.StartContainerResponse{Container: instanceToProto(inst)}), nil
}

func (h *ContainerHandler) StopContainer(ctx context.Context, req *chttp.Request[containersv1.StopContainerRequest]) (*chttp.Response[containersv1.StopContainerResponse], error) {
	inst, err := h.svc.Stop(ctx, req.Msg.Name)
	if err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&containersv1.StopContainerResponse{Container: instanceToProto(inst)}), nil
}

func (h *ContainerHandler) DeleteContainer(ctx context.Context, req *chttp.Request[containersv1.DeleteContainerRequest]) (*chttp.Response[containersv1.DeleteContainerResponse], error) {
	if err := h.svc.Delete(ctx, req.Msg.Name); err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&containersv1.DeleteContainerResponse{}), nil
}

func (h *ContainerHandler) StreamLogs(ctx context.Context, req *chttp.Request[containersv1.StreamLogsRequest], stream *chttp.ServerStream[containersv1.StreamLogsResponse]) error {
	ch, err := h.svc.WatchLogs(ctx, req.Msg.Name)
	if err != nil {
		return toConnectError(err)
	}
	for entry := range ch {
		if err := stream.Send(&containersv1.StreamLogsResponse{
			Line:      entry.Line,
			IsStderr:  entry.IsStderr,
			Timestamp: timestamppb.New(entry.At),
		}); err != nil {
			return err
		}
	}
	return nil
}

func instanceToProto(inst *entity.Instance) *containersv1.Container {
	return &containersv1.Container{
		Id:           inst.ID,
		Name:         inst.Name,
		CreatedAt:    timestamppb.New(inst.CreatedAt),
		DockerId:     inst.DockerID,
		Status:       statusToProto(inst.Status),
		ErrorMessage: inst.ErrorMessage,
	}
}

func statusToProto(s entity.ContainerStatus) containersv1.ContainerStatus {
	switch s {
	case entity.StatusStarting:
		return containersv1.ContainerStatus_CONTAINER_STATUS_STARTING
	case entity.StatusRunning:
		return containersv1.ContainerStatus_CONTAINER_STATUS_RUNNING
	case entity.StatusStopped:
		return containersv1.ContainerStatus_CONTAINER_STATUS_STOPPED
	case entity.StatusError:
		return containersv1.ContainerStatus_CONTAINER_STATUS_ERROR
	default:
		return containersv1.ContainerStatus_CONTAINER_STATUS_UNSPECIFIED
	}
}
