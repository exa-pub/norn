// Package dockerutils provides helper functions on top of the Docker SDK client.
package dockerutils

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// New creates a Docker client configured from environment.
func New() (*client.Client, error) {
	return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
}

// FindByLabel returns the first container matching label=value, or nil.
func FindByLabel(ctx context.Context, cli *client.Client, label, value string) (*types.Container, error) {
	f := filters.NewArgs(filters.Arg("label", label+"="+value))
	list, err := cli.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return nil, fmt.Errorf("docker list: %w", err)
	}
	if len(list) == 0 {
		return nil, nil
	}
	return &list[0], nil
}

// ListByLabel returns all containers that have the given label (any value).
func ListByLabel(ctx context.Context, cli *client.Client, label string) ([]types.Container, error) {
	f := filters.NewArgs(filters.Arg("label", label))
	list, err := cli.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return nil, fmt.Errorf("docker list: %w", err)
	}
	return list, nil
}

// Stop stops a container by ID.
func Stop(ctx context.Context, cli *client.Client, id string) error {
	return cli.ContainerStop(ctx, id, container.StopOptions{})
}

// Remove force-removes a container by ID.
func Remove(ctx context.Context, cli *client.Client, id string) error {
	return cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
}
