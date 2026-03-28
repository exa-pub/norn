package devcontainer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/creack/pty"
)

// --- Up ---

type UpOptions struct {
	Global         *GlobalOptions
	Labels         map[string]string
	Env            map[string]string
	RemoveExisting bool
}

// --- Exec ---

type ExecOptions struct {
	Global   *GlobalOptions
	IDLabels map[string]string
	Cmd      []string
	Cols     uint16
	Rows     uint16
}

// ExecProcess is a running devcontainer exec with a host-side PTY.
type ExecProcess struct {
	PTY *os.File
	Cmd *exec.Cmd
}

func (p *ExecProcess) Resize(cols, rows uint16) error {
	return pty.Setsize(p.PTY, &pty.Winsize{Cols: cols, Rows: rows})
}

func (p *ExecProcess) Close() error {
	_ = p.Cmd.Process.Kill()
	_ = p.Cmd.Wait()
	return p.PTY.Close()
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
	Exec(ctx context.Context, opts ExecOptions) (*ExecProcess, error)
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
	args = append(args, opts.Global.baseArgs()...)
	args = append(args, opts.Global.upArgs()...)

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

func (c *client) Exec(ctx context.Context, opts ExecOptions) (*ExecProcess, error) {
	args := []string{"exec"}
	args = append(args, opts.Global.baseArgs()...)

	for k, v := range opts.IDLabels {
		args = append(args, "--id-label", k+"="+v)
	}
	args = append(args, opts.Cmd...)

	// Use background context — the exec process must outlive the HTTP request
	// that created it. It is terminated explicitly via ExecProcess.Close().
	cmd := exec.Command("devcontainer", args...)
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: opts.Cols, Rows: opts.Rows,
	})
	if err != nil {
		return nil, fmt.Errorf("devcontainer exec: %w", err)
	}

	return &ExecProcess{PTY: ptmx, Cmd: cmd}, nil
}

func (c *client) Run(ctx context.Context, opts RunOptions) ([]byte, []byte, error) {
	args := []string{"exec"}
	args = append(args, opts.Global.baseArgs()...)
	for k, v := range opts.IDLabels {
		args = append(args, "--id-label", k+"="+v)
	}
	args = append(args, opts.Cmd...)

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
