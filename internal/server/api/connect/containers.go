package connect

import (
	"context"

	chttp "connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	containersv1 "github.com/exa-pub/norn/internal/gen/norn/server/containers/v1"
	"github.com/exa-pub/norn/internal/gen/norn/server/containers/v1/containersv1connect"
	"github.com/exa-pub/norn/internal/server/service/instance"
)

var _ containersv1connect.ContainerServiceHandler = (*ContainerHandler)(nil)

type ContainerHandler struct {
	svc instance.Service
}

func NewContainerHandler(svc instance.Service) *ContainerHandler {
	return &ContainerHandler{svc: svc}
}

func (h *ContainerHandler) Create(ctx context.Context, req *chttp.Request[containersv1.CreateRequest]) (*chttp.Response[containersv1.CreateResponse], error) {
	inst, err := h.svc.Create(ctx, req.Msg.Name)
	if err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&containersv1.CreateResponse{Container: instanceToProto(inst)}), nil
}

func (h *ContainerHandler) Start(ctx context.Context, req *chttp.Request[containersv1.StartRequest]) (*chttp.Response[containersv1.StartResponse], error) {
	inst, err := h.svc.Start(ctx, req.Msg.Name, instance.StartOptions{
		RemoveExisting: req.Msg.RemoveExisting,
	})
	if err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&containersv1.StartResponse{Container: instanceToProto(inst)}), nil
}

func (h *ContainerHandler) Stop(ctx context.Context, req *chttp.Request[containersv1.StopRequest]) (*chttp.Response[containersv1.StopResponse], error) {
	inst, err := h.svc.Stop(ctx, req.Msg.Name)
	if err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&containersv1.StopResponse{Container: instanceToProto(inst)}), nil
}

func (h *ContainerHandler) Delete(ctx context.Context, req *chttp.Request[containersv1.DeleteRequest]) (*chttp.Response[containersv1.DeleteResponse], error) {
	if err := h.svc.Delete(ctx, req.Msg.Name); err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&containersv1.DeleteResponse{}), nil
}

func (h *ContainerHandler) Get(ctx context.Context, req *chttp.Request[containersv1.GetRequest]) (*chttp.Response[containersv1.GetResponse], error) {
	inst, err := h.svc.Get(ctx, req.Msg.Name)
	if err != nil {
		return nil, toConnectError(err)
	}
	return chttp.NewResponse(&containersv1.GetResponse{Container: instanceToProto(inst)}), nil
}

func (h *ContainerHandler) List(ctx context.Context, _ *chttp.Request[containersv1.ListRequest]) (*chttp.Response[containersv1.ListResponse], error) {
	list, err := h.svc.List(ctx)
	if err != nil {
		return nil, chttp.NewError(chttp.CodeInternal, err)
	}
	resp := &containersv1.ListResponse{}
	for _, inst := range list {
		resp.Containers = append(resp.Containers, instanceToProto(inst))
	}
	return chttp.NewResponse(resp), nil
}

func (h *ContainerHandler) StreamLogs(ctx context.Context, req *chttp.Request[containersv1.StreamLogsRequest], stream *chttp.ServerStream[containersv1.StreamLogsResponse]) error {
	ch, err := h.svc.WatchLogs(ctx, req.Msg.Name, req.Msg.LogId)
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

func (h *ContainerHandler) ListLogs(ctx context.Context, req *chttp.Request[containersv1.ListLogsRequest]) (*chttp.Response[containersv1.ListLogsResponse], error) {
	files, err := h.svc.ListLogs(req.Msg.Name)
	if err != nil {
		return nil, toConnectError(err)
	}
	resp := &containersv1.ListLogsResponse{}
	for _, f := range files {
		resp.Files = append(resp.Files, &containersv1.LogFile{
			Id:        f.ID,
			Source:    f.Source,
			Date:      f.Date,
			Time:      f.Time,
			SizeBytes: f.Size,
		})
	}
	return chttp.NewResponse(resp), nil
}

func instanceToProto(inst *instance.Instance) *containersv1.Container {
	return &containersv1.Container{
		Id:           inst.ID,
		Name:         inst.Name,
		CreatedAt:    timestamppb.New(inst.CreatedAt),
		DockerId:     inst.DockerID,
		Status:       statusToProto(inst.Status),
		ErrorMessage: inst.ErrorMessage,
	}
}

func statusToProto(s instance.ContainerStatus) containersv1.ContainerStatus {
	switch s {
	case instance.StatusStarting:
		return containersv1.ContainerStatus_CONTAINER_STATUS_STARTING
	case instance.StatusRunning:
		return containersv1.ContainerStatus_CONTAINER_STATUS_RUNNING
	case instance.StatusStopped:
		return containersv1.ContainerStatus_CONTAINER_STATUS_STOPPED
	case instance.StatusError:
		return containersv1.ContainerStatus_CONTAINER_STATUS_ERROR
	default:
		return containersv1.ContainerStatus_CONTAINER_STATUS_UNSPECIFIED
	}
}
