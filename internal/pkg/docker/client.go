package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
)

type Container struct {
	ID     string
	State  string
	Labels map[string]string
}

type Client interface {
	FindByLabel(ctx context.Context, label, value string) (*Container, error)
	ListByLabel(ctx context.Context, label string) ([]*Container, error)
	Stop(ctx context.Context, id string) error
	Remove(ctx context.Context, id string) error
}

func New() (Client, error) {
	dc, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	return &client{dc: dc}, nil
}

type client struct {
	dc *dockerclient.Client
}

func (c *client) Stop(ctx context.Context, id string) error {
	return c.dc.ContainerStop(ctx, id, container.StopOptions{})
}

func (c *client) Remove(ctx context.Context, id string) error {
	return c.dc.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
}

func (c *client) FindByLabel(ctx context.Context, label, value string) (*Container, error) {
	f := filters.NewArgs(filters.Arg("label", label+"="+value))
	list, err := c.dc.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	ct := &list[0]
	return &Container{ID: ct.ID, State: ct.State, Labels: ct.Labels}, nil
}

func (c *client) ListByLabel(ctx context.Context, label string) ([]*Container, error) {
	f := filters.NewArgs(filters.Arg("label", label))
	list, err := c.dc.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return nil, err
	}
	result := make([]*Container, 0, len(list))
	for _, ct := range list {
		result = append(result, &Container{ID: ct.ID, State: ct.State, Labels: ct.Labels})
	}
	return result, nil
}

