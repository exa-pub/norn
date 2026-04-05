// Package daemonconn manages gRPC connections to daemon instances via Unix sockets.
package daemonconn

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"

	"github.com/exa-pub/norn/internal/gen/norn/daemon/agents/v1/agentsv1connect"
	"github.com/exa-pub/norn/internal/gen/norn/daemon/terminals/v1/terminalsv1connect"
	"github.com/exa-pub/norn/internal/gen/norn/daemon/tty/v1/ttyv1connect"
)

// Conn holds ConnectRPC clients for a single daemon instance.
type Conn struct {
	Agents    agentsv1connect.AgentServiceClient
	Terminals terminalsv1connect.TerminalServiceClient
	TTY       ttyv1connect.TTYServiceClient
}

// Pool manages connections to daemon instances keyed by instance name.
type Pool struct {
	storageDir string

	mu    sync.Mutex
	conns map[string]*Conn
}

func NewPool(storageDir string) *Pool {
	return &Pool{
		storageDir: storageDir,
		conns:      make(map[string]*Conn),
	}
}

// Get returns a cached connection or creates a new one.
// If the daemon socket file has disappeared (daemon died), the cached
// connection is evicted so a fresh one is created on the next call.
func (p *Pool) Get(instanceName string) (*Conn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	socketPath := filepath.Join(p.storageDir, "instances", instanceName, "run", "daemon.sock")

	if c, ok := p.conns[instanceName]; ok {
		// Verify socket still exists — evict stale connections for dead daemons.
		if _, err := os.Stat(socketPath); err != nil {
			delete(p.conns, instanceName)
			return nil, fmt.Errorf("%w: socket gone: %v", ErrDaemonUnavailable, err)
		}
		return c, nil
	}

	// Create HTTP/2 cleartext client that dials Unix socket.
	// h2c is required because the daemon serves gRPC over h2c.
	dialer := net.Dialer{}
	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, _, _ string, _ *tls.Config) (net.Conn, error) {
				return dialer.DialContext(ctx, "unix", socketPath)
			},
		},
	}

	// Base URL host is required by ConnectRPC but ignored for Unix sockets.
	// The /connect prefix matches the daemon's chi.Mount("/connect", ...) route.
	baseURL := "http://localhost/connect"
	opts := connect.WithGRPC()

	c := &Conn{
		Agents:    agentsv1connect.NewAgentServiceClient(httpClient, baseURL, opts),
		Terminals: terminalsv1connect.NewTerminalServiceClient(httpClient, baseURL, opts),
		TTY:       ttyv1connect.NewTTYServiceClient(httpClient, baseURL, opts),
	}

	p.conns[instanceName] = c
	return c, nil
}

// Remove closes and removes a cached connection.
func (p *Pool) Remove(instanceName string) {
	p.mu.Lock()
	delete(p.conns, instanceName)
	p.mu.Unlock()
}

// SocketPath returns the expected socket path for an instance.
func (p *Pool) SocketPath(instanceName string) string {
	return filepath.Join(p.storageDir, "instances", instanceName, "run", "daemon.sock")
}

// ErrDaemonUnavailable is returned when the daemon is not reachable.
var ErrDaemonUnavailable = fmt.Errorf("daemon unavailable")
