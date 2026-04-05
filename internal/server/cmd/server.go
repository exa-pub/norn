package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/exa-pub/norn/internal/gen/norn/server/agents/v1/agentsv1connect"
	"github.com/exa-pub/norn/internal/gen/norn/server/containers/v1/containersv1connect"
	"github.com/exa-pub/norn/internal/gen/norn/server/terminals/v1/terminalsv1connect"
	"github.com/exa-pub/norn/internal/pkg/dockerutils"
	"github.com/exa-pub/norn/internal/pkg/envflag"
	"github.com/exa-pub/norn/internal/pkg/httpsrv"
	"github.com/exa-pub/norn/internal/pkg/logging"
	"github.com/exa-pub/norn/internal/pkg/logging/chilogging"
	nornconnect "github.com/exa-pub/norn/internal/server/api/connect"
	"github.com/exa-pub/norn/internal/server/api/middleware"
	"github.com/exa-pub/norn/internal/server/api/ws"
	"github.com/exa-pub/norn/internal/server/service/agent"
	"github.com/exa-pub/norn/internal/server/service/daemonconn"
	"github.com/exa-pub/norn/internal/server/service/devcontainer"
	"github.com/exa-pub/norn/internal/server/service/instance"
	"github.com/exa-pub/norn/internal/server/service/storage"
	"github.com/exa-pub/norn/internal/server/service/terminal"
	"github.com/exa-pub/norn/web"
)

func Command() *cobra.Command {
	var (
		addr        string
		storageDir  string
		secret      string
		dcMountPath string
		dcOpts      devcontainer.GlobalOptions
	)

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start the Norn server",
		PreRun: func(cmd *cobra.Command, args []string) {
			envflag.BindAll(cmd, map[string]string{
				"addr":                     "NORN_ADDR",
				"storage-dir":              "NORN_STORAGE_DIR",
				"auth-secret":              "NORN_SECRET",
				"norn-dc-mount-path":       "NORN_DC_MOUNT_PATH",
				"workspace-folder":         "NORN_WORKSPACE_FOLDER",
				"config":                   "NORN_CONFIG",
				"override-config":          "NORN_OVERRIDE_CONFIG",
				"docker-path":              "NORN_DOCKER_PATH",
				"dotfiles-repository":      "NORN_DOTFILES_REPO",
				"dotfiles-install-command": "NORN_DOTFILES_COMMAND",
				"dotfiles-target-path":     "NORN_DOTFILES_PATH",
				"secrets-file":             "NORN_SECRETS_FILE",
			})
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Secret priority: flag/env (already resolved by envflag) > file > generate
			secretFile := filepath.Join(storageDir, "secret")
			if secret == "" {
				if data, err := os.ReadFile(secretFile); err == nil {
					secret = strings.TrimSpace(string(data))
				}
			}
			if secret == "" {
				generated, err := middleware.GenerateSecret()
				if err != nil {
					return fmt.Errorf("generate secret: %w", err)
				}
				secret = generated
			}
			_ = os.MkdirAll(storageDir, 0o755)
			_ = os.WriteFile(secretFile, []byte(secret+"\n"), 0o600)

			return serve(addr, storageDir, secret, dcMountPath, &dcOpts)
		},
		SilenceUsage: true,
	}

	f := cmd.Flags()
	f.StringVar(&addr, "addr", ":8080", "listen address")
	f.StringVar(&storageDir, "storage-dir", ".norn", "NornHome directory")
	f.StringVar(&secret, "auth-secret", "", "authentication secret (default: $NORN_SECRET or auto-generated)")
	f.StringVar(&dcMountPath, "norn-dc-mount-path", "/mnt/norn/", "mount path inside devcontainers")
	f.StringVar(&dcOpts.WorkspaceFolder, "workspace-folder", ".", "devcontainer workspace folder")
	f.StringVar(&dcOpts.Config, "config", "", "devcontainer.json path")
	f.StringVar(&dcOpts.OverrideConfig, "override-config", "", "devcontainer.json path to override workspace config")
	f.StringVar(&dcOpts.DockerPath, "docker-path", "", "Docker CLI path")
	f.StringToStringVar(&dcOpts.RemoteEnv, "remote-env", nil, "remote environment variables (key=value)")
	f.StringArrayVar(&dcOpts.Mounts, "mount", nil, "additional mount points")
	f.StringVar(&dcOpts.DotfilesRepo, "dotfiles-repository", "", "dotfiles Git repository URL")
	f.StringVar(&dcOpts.DotfilesCommand, "dotfiles-install-command", "", "dotfiles install command")
	f.StringVar(&dcOpts.DotfilesPath, "dotfiles-target-path", "", "dotfiles target path")
	f.StringVar(&dcOpts.SecretsFile, "secrets-file", "", "path to secrets JSON file")

	return cmd
}

func serve(addr, storageDir, secret, dcMountPath string, dcOpts *devcontainer.GlobalOptions) error {
	logging.Init(true) // development (human-readable) format
	defer logging.Sync()

	store := storage.NewFileStore(storageDir)
	dk, err := dockerutils.New()
	if err != nil {
		return fmt.Errorf("docker: %w", err)
	}
	dc := devcontainer.New()

	pool := daemonconn.NewPool(store.BaseDir())
	instanceSvc := instance.NewService(store, store, dc, dk, dcOpts, dcMountPath)
	agentSvc := agent.NewService(pool)
	terminalSvc := terminal.NewService(pool)

	r := chi.NewRouter()
	r.Use(chimiddleware.Recoverer)
	r.Use(chilogging.Middleware(zap.L()))

	r.Group(func(api chi.Router) {
		api.Use(middleware.Auth(secret))

		api.Route("/connect", func(cr chi.Router) {
			cr.Use(syncRoutePath)

			{
				path, handler := containersv1connect.NewContainerServiceHandler(
					nornconnect.NewContainerHandler(instanceSvc),
				)
				cr.Mount(path, handler)
			}
			{
				path, handler := agentsv1connect.NewAgentServiceHandler(
					nornconnect.NewAgentHandler(agentSvc),
				)
				cr.Mount(path, handler)
			}
			{
				path, handler := terminalsv1connect.NewTerminalServiceHandler(
					nornconnect.NewTerminalHandler(terminalSvc),
				)
				cr.Mount(path, handler)
			}
		})

		// WebSocket PTY proxy
		api.Handle("/ws/*", ws.Handler(pool))
	})

	webContent, _ := fs.Sub(web.DistFS, "dist")
	fileServer := http.FileServer(http.FS(webContent))
	r.Handle("/*", fileServer)

	srv := httpsrv.New(r)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		host := addr
		if host[0] == ':' {
			host = "localhost" + host
		}
		zap.L().Info("server started", zap.String("url", fmt.Sprintf("http://%s/#nornSecret=%s", host, secret)))
		if err := srv.ListenAndServeTCP(addr); err != nil && err != http.ErrServerClosed {
			zap.L().Fatal("listen failed", zap.Error(err))
		}
	}()

	<-ctx.Done()
	zap.L().Info("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	instanceSvc.Shutdown()
	zap.L().Info("stopped")

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
