package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/spf13/cobra"

	nornconnect "github.com/exa-pub/norn/internal/api/connect"
	"github.com/exa-pub/norn/internal/api/middleware"
	"github.com/exa-pub/norn/internal/api/ws"
	"github.com/exa-pub/norn/internal/gen/norn/agents/v1/agentsv1connect"
	"github.com/exa-pub/norn/internal/gen/norn/containers/v1/containersv1connect"
	"github.com/exa-pub/norn/internal/gen/norn/terminals/v1/terminalsv1connect"
	"github.com/exa-pub/norn/internal/pkg/devcontainer"
	"github.com/exa-pub/norn/internal/service/agent"
	"github.com/exa-pub/norn/internal/service/instance"
	"github.com/exa-pub/norn/internal/service/storage"
	"github.com/exa-pub/norn/internal/service/terminal"
	"github.com/exa-pub/norn/internal/service/tty"
	"github.com/exa-pub/norn/pkg/dockerutils"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	var (
		addr       string
		storageDir string
		secret     string
		dcOpts     devcontainer.GlobalOptions
	)

	cmd := &cobra.Command{
		Use:   "norn",
		Short: "Norn — AI agent devcontainer manager",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve secret: flag > env > generate.
			if secret == "" {
				secret = os.Getenv("NORN_SECRET")
			}
			if secret == "" {
				generated, err := middleware.GenerateSecret()
				if err != nil {
					return fmt.Errorf("generate secret: %w", err)
				}
				secret = generated
			}

			return serve(addr, storageDir, secret, &dcOpts)
		},
		SilenceUsage: true,
	}

	f := cmd.Flags()

	// Norn flags
	f.StringVar(&addr, "addr", ":8080", "listen address")
	f.StringVar(&storageDir, "storage-dir", ".norn", "NornHome directory")
	f.StringVar(&secret, "auth-secret", "", "authentication secret (default: $NORN_SECRET or auto-generated)")

	// Devcontainer flags
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

func serve(addr, storageDir, secret string, dcOpts *devcontainer.GlobalOptions) error {
	store := storage.NewFileStore(storageDir)
	dk, err := dockerutils.New()
	if err != nil {
		return fmt.Errorf("docker: %w", err)
	}
	dc := devcontainer.New()

	instanceSvc := instance.NewService(store, store, dc, dk, dcOpts)
	ttyMgr := tty.NewManager(dc, dcOpts)
	terminalSvc := terminal.NewService(ttyMgr)
	agentSvc := agent.NewService(store, store, ttyMgr, dk)

	r := chi.NewRouter()

	// Middleware
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.Auth(secret))

	// ConnectRPC services
	{
		path, handler := containersv1connect.NewContainerServiceHandler(
			nornconnect.NewContainerHandler(instanceSvc),
		)
		r.Handle(path+"*", handler)
	}
	{
		path, handler := agentsv1connect.NewAgentServiceHandler(
			nornconnect.NewAgentHandler(agentSvc),
		)
		r.Handle(path+"*", handler)
	}
	{
		path, handler := terminalsv1connect.NewTerminalServiceHandler(
			nornconnect.NewTerminalHandler(terminalSvc),
		)
		r.Handle(path+"*", handler)
	}

	// WebSocket PTY bridge
	r.Handle("/ws/*", ws.Handler(ttyMgr))

	srv := &http.Server{Addr: addr, Handler: r}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		host := addr
		if host[0] == ':' {
			host = "localhost" + host
		}
		log.Printf("http://%s/#nornSecret=%s", host, secret)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	instanceSvc.Shutdown()
	log.Println("stopped")

	return nil
}
