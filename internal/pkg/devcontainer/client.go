package devcontainer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// UpOptions configures a devcontainer up invocation.
type UpOptions struct {
	WorkspaceFolder string
	ConfigPath      string
	Labels          map[string]string // --id-label key=value (each pair)
	Env             map[string]string // extra env vars for the process
	RemoveExisting  bool              // --remove-existing-container
}

// Client wraps the devcontainer CLI. No norn-specific knowledge.
type Client interface {
	// Up runs `devcontainer up` with the given options.
	// Stdout and stderr of the process are written to the provided writers.
	// Returns the Docker container ID on success.
	Up(ctx context.Context, opts UpOptions, stdout, stderr io.Writer) (dockerID string, err error)
}

// New creates a Client backed by the devcontainer CLI binary.
func New() Client {
	return &client{}
}

type client struct{}

func (c *client) Up(ctx context.Context, opts UpOptions, stdout, stderr io.Writer) (string, error) {
	args := []string{"up",
		"--workspace-folder", opts.WorkspaceFolder,
		"--override-config", opts.ConfigPath,
	}
	for k, v := range opts.Labels {
		args = append(args, "--id-label", k+"="+v)
	}
	if opts.RemoveExisting {
		args = append(args, "--remove-existing-container")
	}

	cmd := exec.CommandContext(ctx, "devcontainer", args...)
	env := os.Environ()
	for k, v := range opts.Env {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	// Capture stdout for parsing containerId while also copying to the caller's writer.
	var stdoutBuf strings.Builder
	cmd.Stdout = io.MultiWriter(&stdoutBuf, stdout)
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("devcontainer up: %w", err)
	}

	dockerID, err := parseContainerID(stdoutBuf.String())
	if err != nil {
		return "", fmt.Errorf("parse container id: %w", err)
	}
	return dockerID, nil
}

type devcontainerResult struct {
	ContainerID string `json:"containerId"`
}

func parseContainerID(output string) (string, error) {
	lines := strings.Split(output, "\n")
	// Try from end — devcontainers/cli prints JSON as the last line.
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var result devcontainerResult
		if err := json.Unmarshal([]byte(line), &result); err == nil && result.ContainerID != "" {
			return result.ContainerID, nil
		}
		break
	}
	for _, line := range lines {
		var result devcontainerResult
		if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &result); err == nil && result.ContainerID != "" {
			return result.ContainerID, nil
		}
	}
	return "", fmt.Errorf("containerId not found in devcontainer output")
}
