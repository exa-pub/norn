// Package httpsrv provides a reusable HTTP/h2c server with graceful shutdown.
// It supports both TCP and Unix socket listeners.
package httpsrv

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// Server wraps http.Server with h2c support and graceful shutdown.
type Server struct {
	httpSrv *http.Server
}

// New creates a server with the given handler.
// The handler is wrapped with h2c for HTTP/2 cleartext support
// (required for gRPC/ConnectRPC bidi streaming without TLS).
func New(handler http.Handler) *Server {
	return &Server{
		httpSrv: &http.Server{
			Handler: h2c.NewHandler(handler, &http2.Server{}),
		},
	}
}

// ListenAndServeTCP starts the server on a TCP address (e.g. ":8080").
func (s *Server) ListenAndServeTCP(addr string) error {
	s.httpSrv.Addr = addr
	return s.httpSrv.ListenAndServe()
}

// ListenAndServeUnix starts the server on a Unix socket.
// Removes stale socket file before listening.
func (s *Server) ListenAndServeUnix(socketPath string) error {
	_ = os.Remove(socketPath)

	lis, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("listen unix %s: %w", socketPath, err)
	}
	return s.httpSrv.Serve(lis)
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}
