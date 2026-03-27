package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	nornconnect "github.com/exa-pub/norn/internal/api/connect"
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
	addr := flag.String("addr", ":8080", "listen address")
	storageDir := flag.String("storage-dir", ".norn", "NornHome directory")
	workspaceFolder := flag.String("workspace-folder", ".", "devcontainer workspace folder")
	configPath := flag.String("devcontainer-config", ".devcontainer/devcontainer.json", "devcontainer config path")
	flag.Parse()

	store := storage.NewFileStore(*storageDir)
	dk, err := dockerutils.New()
	if err != nil {
		log.Fatalf("docker: %v", err)
	}
	dc := devcontainer.New()

	instanceSvc := instance.NewService(store, store, dc, dk, instance.Config{
		WorkspaceFolder: *workspaceFolder,
		ConfigPath:      *configPath,
	})

	ttyMgr := tty.NewManager(dc, tty.Config{
		WorkspaceFolder: *workspaceFolder,
		ConfigPath:      *configPath,
	})
	terminalSvc := terminal.NewService(ttyMgr)
	agentSvc := agent.NewService(store, store, ttyMgr)

	mux := http.NewServeMux()
	{
		path, handler := containersv1connect.NewContainerServiceHandler(
			nornconnect.NewContainerHandler(instanceSvc),
		)
		mux.Handle(path, handler)
	}
	{
		path, handler := agentsv1connect.NewAgentServiceHandler(
			nornconnect.NewAgentHandler(agentSvc),
		)
		mux.Handle(path, handler)
	}
	{
		path, handler := terminalsv1connect.NewTerminalServiceHandler(
			nornconnect.NewTerminalHandler(terminalSvc),
		)
		mux.Handle(path, handler)
	}
	mux.Handle("/ws/", ws.Handler(ttyMgr))

	srv := &http.Server{Addr: *addr, Handler: mux}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("listening on %s", *addr)
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
}
