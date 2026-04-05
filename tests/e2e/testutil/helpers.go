package testutil

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/net/http2"
)

const DefaultNornImage = "ghcr.io/exa-pub/norn:latest"

// NornImage returns the norn Docker image to use for e2e tests.
// Override with NORN_E2E_IMAGE env var to test a specific tag.
func NornImage() string {
	if img := os.Getenv("NORN_E2E_IMAGE"); img != "" {
		return img
	}
	return DefaultNornImage
}

// NornContainerResult holds the started norn container and its base URL.
type NornContainerResult struct {
	Container testcontainers.Container
	BaseURL   string // e.g. "http://localhost:32781/connect"
}

// StartNornContainer creates and starts a norn server container via testcontainers.
// workspacePath is the host path to mount as /workspace (e.g. examples/test).
func StartNornContainer(ctx context.Context, secret, workspacePath string) (*NornContainerResult, error) {
	req := testcontainers.ContainerRequest{
		Image:        NornImage(),
		Privileged:   true,
		ExposedPorts: []string{"8080/tcp"},
		Env: map[string]string{
			"NORN_SECRET":           secret,
			"NORN_WORKSPACE_FOLDER": "/workspace",
			"NORN_STORAGE_DIR":      "/data/.norn",
		},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.Privileged = true
			hc.Mounts = append(hc.Mounts,
				mount.Mount{
					Type:   mount.TypeBind,
					Source: workspacePath,
					Target: "/workspace",
				},
			)
		},
		WaitingFor: wait.ForHTTP("/").
			WithPort("8080").
			WithStartupTimeout(4 * time.Minute),
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("start norn container: %w", err)
	}

	mappedPort, err := c.MappedPort(ctx, "8080")
	if err != nil {
		return nil, fmt.Errorf("mapped port: %w", err)
	}
	host, err := c.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("host: %w", err)
	}

	baseURL := fmt.Sprintf("http://%s:%s/connect", host, mappedPort.Port())
	return &NornContainerResult{Container: c, BaseURL: baseURL}, nil
}

// NornDaemonResult holds the started daemon container and socket path on the host.
type NornDaemonResult struct {
	Container  testcontainers.Container
	SocketPath string // host-side path to daemon.sock
	RunDir     string // host-side temp dir (caller should os.RemoveAll)
}

// StartNornDaemon starts a norn daemon inside a container.
// mockClaudeDir is the host path to a directory containing a mock `claude` script.
// The daemon listens on a Unix socket exposed via a bind-mounted temp directory.
func StartNornDaemon(ctx context.Context, mockClaudeDir string) (*NornDaemonResult, error) {
	runDir, err := os.MkdirTemp("", "norn-daemon-e2e-*")
	if err != nil {
		return nil, fmt.Errorf("mkdtemp: %w", err)
	}

	const (
		containerRunDir  = "/mnt/norn/run"
		containerSocket  = containerRunDir + "/daemon.sock"
		containerMockDir = "/opt/mock-bin"
	)

	req := testcontainers.ContainerRequest{
		Image: NornImage(),
		Entrypoint: []string{
			"sh", "-c",
			fmt.Sprintf(
				"export PATH=%s:$PATH && mkdir -p %s && norn run daemon --socket %s --run-dir %s &"+
					" while [ ! -S %s ]; do sleep 0.1; done && chmod 777 %s && wait",
				containerMockDir, containerRunDir, containerSocket, containerRunDir,
				containerSocket, containerSocket,
			),
		},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.Mounts = append(hc.Mounts,
				mount.Mount{
					Type:   mount.TypeBind,
					Source: runDir,
					Target: containerRunDir,
				},
				mount.Mount{
					Type:     mount.TypeBind,
					Source:   mockClaudeDir,
					Target:   containerMockDir,
					ReadOnly: true,
				},
			)
		},
		WaitingFor: wait.ForExec([]string{"test", "-S", containerSocket}).
			WithStartupTimeout(30 * time.Second),
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		os.RemoveAll(runDir)
		return nil, fmt.Errorf("start daemon container: %w", err)
	}

	socketPath := fmt.Sprintf("%s/daemon.sock", runDir)

	// Wait for socket to be reachable from the host.
	if err := WaitForSocket(ctx, socketPath); err != nil {
		c.Terminate(context.Background())
		os.RemoveAll(runDir)
		return nil, err
	}

	return &NornDaemonResult{
		Container:  c,
		SocketPath: socketPath,
		RunDir:     runDir,
	}, nil
}

// UniqueName generates a unique instance name for tests.
func UniqueName(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("t-%d", time.Now().UnixNano()%1000000)
}

// UnixHTTPClient returns an *http.Client that dials the given Unix socket
// using HTTP/2 cleartext (h2c), as required by ConnectRPC bidi streams.
func UnixHTTPClient(socketPath string) *http.Client {
	return &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socketPath)
			},
		},
	}
}

// AuthHTTPClient wraps an http.Client to inject the norn_secret cookie.
func AuthHTTPClient(base *http.Client, secret string) *http.Client {
	return &http.Client{
		Transport: &authTransport{
			base:   base.Transport,
			secret: secret,
		},
	}
}

type authTransport struct {
	base   http.RoundTripper
	secret string
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.AddCookie(&http.Cookie{Name: "norn_secret", Value: t.secret})
	return t.base.RoundTrip(req)
}

// WaitForSocket polls until the Unix socket is reachable or the context is cancelled.
func WaitForSocket(ctx context.Context, socketPath string) error {
	for {
		conn, err := net.Dial("unix", socketPath)
		if err == nil {
			conn.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for socket %s: %w", socketPath, ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// H2CClient returns an http.Client configured for HTTP/2 cleartext (h2c).
func H2CClient() *http.Client {
	return &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, addr)
			},
		},
	}
}
