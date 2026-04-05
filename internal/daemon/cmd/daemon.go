package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	daemonconnect "github.com/exa-pub/norn/internal/daemon/api/connect"
	"github.com/exa-pub/norn/internal/daemon/service/agent"
	"github.com/exa-pub/norn/internal/daemon/service/storage"
	"github.com/exa-pub/norn/internal/daemon/service/terminal"
	"github.com/exa-pub/norn/internal/daemon/service/tty"
	"github.com/exa-pub/norn/internal/gen/norn/daemon/agents/v1/agentsv1connect"
	"github.com/exa-pub/norn/internal/gen/norn/daemon/terminals/v1/terminalsv1connect"
	"github.com/exa-pub/norn/internal/gen/norn/daemon/tty/v1/ttyv1connect"
	"github.com/exa-pub/norn/internal/pkg/envflag"
	"github.com/exa-pub/norn/internal/pkg/httpsrv"
	"github.com/exa-pub/norn/internal/pkg/logging"
	"github.com/exa-pub/norn/internal/pkg/logging/chilogging"
)

func Command() *cobra.Command {
	var (
		socketPath string
		runDir     string
	)

	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Run daemon inside container (listens on Unix socket)",
		PreRun: func(cmd *cobra.Command, args []string) {
			envflag.BindAll(cmd, map[string]string{
				"socket":  "NORN_DAEMON_SOCKET",
				"run-dir": "NORN_DAEMON_RUN_DIR",
			})
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(socketPath, runDir)
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVar(&socketPath, "socket", "/mnt/norn/run/daemon.sock", "path to Unix socket")
	cmd.Flags().StringVar(&runDir, "run-dir", "/mnt/norn/run", "daemon run directory (for agent metadata)")

	return cmd
}

func run(socketPath, runDir string) error {
	logging.Init(false) // production (JSON) format
	defer logging.Sync()

	store := storage.New(runDir)
	if err := store.EnsureDirs(); err != nil {
		return fmt.Errorf("ensure dirs: %w", err)
	}

	ttyMgr := tty.NewManager()
	agentSvc := agent.NewService(store, ttyMgr)
	terminalSvc := terminal.NewService(ttyMgr)

	r := chi.NewRouter()
	r.Use(chimiddleware.Recoverer)
	r.Use(chilogging.Middleware(zap.L()))

	r.Route("/connect", func(cr chi.Router) {
		cr.Use(syncRoutePath)

		{
			path, handler := agentsv1connect.NewAgentServiceHandler(
				daemonconnect.NewAgentHandler(agentSvc),
			)
			cr.Mount(path, handler)
		}
		{
			path, handler := terminalsv1connect.NewTerminalServiceHandler(
				daemonconnect.NewTerminalHandler(terminalSvc),
			)
			cr.Mount(path, handler)
		}
		{
			path, handler := ttyv1connect.NewTTYServiceHandler(
				daemonconnect.NewTTYHandler(ttyMgr),
			)
			cr.Mount(path, handler)
		}
	})

	srv := httpsrv.New(r)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		zap.L().Info("norn daemon listening", zap.String("socket", socketPath))
		if err := srv.ListenAndServeUnix(socketPath); err != nil && err != http.ErrServerClosed {
			zap.L().Fatal("daemon serve failed", zap.Error(err))
		}
	}()

	<-ctx.Done()
	zap.L().Info("norn daemon shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)

	zap.L().Info("norn daemon stopped")
	return nil
}

// syncRoutePath propagates chi's rctx.RoutePath into r.URL.Path.
// chi.Mount only updates the routing context, but Connect RPC handlers
// read r.URL.Path directly — without this middleware they see the
// un-stripped path and return 404.
func syncRoutePath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		if rctx != nil && rctx.RoutePath != "" {
			r.URL.Path = rctx.RoutePath
			r.URL.RawPath = ""
		}
		next.ServeHTTP(w, r)
	})
}
