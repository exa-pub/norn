package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	nornconnect "github.com/exa-pub/norn/internal/connect"
	"github.com/exa-pub/norn/internal/container"
	"github.com/exa-pub/norn/internal/gen/norn/containers/v1/containersv1connect"
	"github.com/exa-pub/norn/internal/pkg/devcontainer"
	"github.com/exa-pub/norn/internal/pkg/docker"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	workspaceFolder := flag.String("workspace-folder", ".", "devcontainer workspace folder")
	configPath := flag.String("devcontainer-config", ".devcontainer/devcontainer.json", "devcontainer config path")
	storageDir := flag.String("storage-dir", "./storage", "directory for instance data")
	flag.Parse()

	dc := devcontainer.New()
	dk, err := docker.New()
	if err != nil {
		log.Fatalf("docker client: %v", err)
	}
	mgr := container.NewManager(dc, dk, container.Config{
		WorkspaceFolder: *workspaceFolder,
		ConfigPath:      *configPath,
		StorageDir:      *storageDir,
	})

	mux := http.NewServeMux()
	path, handler := containersv1connect.NewContainerServiceHandler(
		nornconnect.NewContainerHandler(mgr),
	)
	mux.Handle(path, handler)

	srv := &http.Server{Addr: *addr, Handler: mux}

	// Listen for shutdown signals.
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

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown: %v", err)
	}
	mgr.Shutdown()
	log.Println("stopped")
}
