package docker

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
)

// Container holds metadata about a Docker container.
type Container struct {
	ID        string
	State     string            // "running", "exited", "created", etc.
	Labels    map[string]string
	CreatedAt time.Time
}

// Client wraps the Docker Engine API. No norn-specific knowledge.
type Client interface {
	Stop(ctx context.Context, id string) error
	Remove(ctx context.Context, id string) error
	// FindByLabel returns the first container matching label=value. Nil if not found.
	FindByLabel(ctx context.Context, label, value string) (*Container, error)
	// ListByLabel returns all containers that have the given label (any value).
	ListByLabel(ctx context.Context, label string) ([]*Container, error)
}

// New creates a Client backed by the Docker Engine API.
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
	if err := c.dc.ContainerStop(ctx, id, container.StopOptions{}); err != nil {
		return fmt.Errorf("docker stop %s: %w", id, err)
	}
	return nil
}

func (c *client) Remove(ctx context.Context, id string) error {
	if err := c.dc.ContainerRemove(ctx, id, container.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("docker rm %s: %w", id, err)
	}
	return nil
}

func (c *client) FindByLabel(ctx context.Context, label, value string) (*Container, error) {
	f := filters.NewArgs(filters.Arg("label", label+"="+value))
	list, err := c.dc.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return nil, fmt.Errorf("docker list: %w", err)
	}
	if len(list) == 0 {
		return nil, nil
	}
	ct := &list[0]
	return &Container{
		ID:        ct.ID,
		State:     ct.State,
		Labels:    ct.Labels,
		CreatedAt: time.Unix(ct.Created, 0),
	}, nil
}

func (c *client) ListByLabel(ctx context.Context, label string) ([]*Container, error) {
	f := filters.NewArgs(filters.Arg("label", label))
	list, err := c.dc.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return nil, fmt.Errorf("docker list: %w", err)
	}
	result := make([]*Container, 0, len(list))
	for _, ct := range list {
		result = append(result, &Container{
			ID:        ct.ID,
			State:     ct.State,
			Labels:    ct.Labels,
			CreatedAt: time.Unix(ct.Created, 0),
		})
	}
	return result, nil
}
