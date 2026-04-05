package devcontainer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

// --- Up ---

type UpOptions struct {
	Global         *GlobalOptions
	Labels         map[string]string
	Env            map[string]string
	Mounts         []string // instance-specific mounts (appended to Global.Mounts)
	RemoveExisting bool
}

// RunOptions describes a non-PTY exec (fire-and-forget or collect output).
type RunOptions struct {
	Global   *GlobalOptions
	IDLabels map[string]string
	Cmd      []string
	Stdin    io.Reader // optional; nil → closed immediately
}

// --- Client ---

type Client interface {
	Up(ctx context.Context, opts UpOptions, stdout, stderr io.Writer) (dockerID string, err error)
	// Run executes a command inside the container without a PTY.
	// Returns combined stdout and the error (including non-zero exit).
	Run(ctx context.Context, opts RunOptions) (stdout []byte, stderr []byte, err error)
}

func New() Client {
	return &client{}
}

type client struct{}

func (c *client) Up(ctx context.Context, opts UpOptions, stdout, stderr io.Writer) (string, error) {
	args := []string{"up"}
	args = append(args, opts.Global.BaseArgs()...)
	args = append(args, opts.Global.upArgs()...)

	for _, m := range opts.Mounts {
		args = append(args, "--mount", m)
	}
	for k, v := range opts.Labels {
		args = append(args, "--id-label", k+"="+v)
	}
	if opts.RemoveExisting {
		args = append(args, "--remove-existing-container")
	}

	zap.L().Info("devcontainer up", zap.Strings("args", args))
	cmd := exec.CommandContext(ctx, "devcontainer", args...)
	env := os.Environ()
	for k, v := range opts.Env {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

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

func (c *client) Run(ctx context.Context, opts RunOptions) ([]byte, []byte, error) {
	args := []string{"exec"}
	args = append(args, opts.Global.BaseArgs()...)
	for k, v := range opts.IDLabels {
		args = append(args, "--id-label", k+"="+v)
	}
	args = append(args, opts.Cmd...)

	zap.L().Info("devcontainer exec", zap.Strings("args", args))
	cmd := exec.CommandContext(ctx, "devcontainer", args...)
	if opts.Stdin != nil {
		cmd.Stdin = opts.Stdin
	}

	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	return []byte(stdoutBuf.String()), []byte(stderrBuf.String()), err
}

// --- parsing ---

type devcontainerResult struct {
	ContainerID string `json:"containerId"`
}

func parseContainerID(output string) (string, error) {
	lines := strings.Split(output, "\n")
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
