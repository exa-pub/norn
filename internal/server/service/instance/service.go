package instance

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/exa-pub/norn/internal/pkg/dockerutils"
	"github.com/exa-pub/norn/internal/server/service/devcontainer"
	"github.com/exa-pub/norn/internal/server/service/storage"
)

var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]{0,62}$`)

type StartOptions struct {
	RemoveExisting bool
}

type Service interface {
	Create(ctx context.Context, name string) (*Instance, error)
	Get(ctx context.Context, name string) (*Instance, error)
	List(ctx context.Context) ([]*Instance, error)
	Start(ctx context.Context, name string, opts StartOptions) (*Instance, error)
	Stop(ctx context.Context, name string) (*Instance, error)
	Delete(ctx context.Context, name string) error
	WatchLogs(ctx context.Context, name string, logID string) (<-chan LogEntry, error)
	ListLogs(name string) ([]storage.LogFileInfo, error)
	Shutdown()
}

type activeEntry struct {
	writer *LogWriter
}

type service struct {
	store         storage.InstanceStore
	home          storage.Home
	dc            devcontainer.Client
	docker        *client.Client
	opts          *devcontainer.GlobalOptions
	dcMountPath string // mount path inside devcontainers (default "/mnt/norn/")

	mu     sync.Mutex
	active map[string]*activeEntry

	cancel context.CancelFunc
	bgCtx  context.Context
	wg     sync.WaitGroup
}

func NewService(store storage.InstanceStore, home storage.Home, dc devcontainer.Client, dk *client.Client, opts *devcontainer.GlobalOptions, dcMountPath string) Service {
	if dcMountPath == "" {
		dcMountPath = "/mnt/norn/"
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &service{
		store:         store,
		home:          home,
		dc:            dc,
		docker:        dk,
		opts:          opts,
		dcMountPath: dcMountPath,
		active:        make(map[string]*activeEntry),
		bgCtx:         ctx,
		cancel:        cancel,
	}
}

func (s *service) Create(ctx context.Context, name string) (*Instance, error) {
	if !validName.MatchString(name) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidName, name)
	}
	if s.store.Exists(name) {
		return nil, fmt.Errorf("instance %q: %w", name, ErrAlreadyExists)
	}

	now := time.Now()
	meta := storage.InstanceMeta{
		ID:        uuid.New().String(),
		Name:      name,
		CreatedAt: now,
	}

	if err := s.home.EnsureInstanceDirs(name); err != nil {
		return nil, err
	}
	if err := s.store.Create(name, meta); err != nil {
		return nil, err
	}

	return &Instance{
		ID: meta.ID, Name: name, CreatedAt: now,
		Status: StatusStopped,
	}, nil
}

func (s *service) Get(ctx context.Context, name string) (*Instance, error) {
	meta, err := s.store.Read(name)
	if err != nil {
		return nil, fmt.Errorf("instance %q: %w", name, ErrNotFound)
	}

	s.mu.Lock()
	_, starting := s.active[name]
	s.mu.Unlock()

	dc, err := dockerutils.FindByLabel(ctx, s.docker, "norn.id", meta.ID)
	if err != nil {
		return nil, fmt.Errorf("docker: %w", err)
	}

	lastError := s.store.ReadError(name)
	return buildInstance(meta, dc, starting, lastError), nil
}

func (s *service) List(ctx context.Context) ([]*Instance, error) {
	names, err := s.store.List()
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	activeNames := make(map[string]bool, len(s.active))
	for name := range s.active {
		activeNames[name] = true
	}
	s.mu.Unlock()

	dcs, err := dockerutils.ListByLabel(ctx, s.docker, "norn.id")
	if err != nil {
		return nil, err
	}
	dockerByID := make(map[string]*types.Container, len(dcs))
	for i := range dcs {
		if id := dcs[i].Labels["norn.id"]; id != "" {
			dockerByID[id] = &dcs[i]
		}
	}

	var result []*Instance
	for _, name := range names {
		meta, err := s.store.Read(name)
		if err != nil {
			continue
		}
		dc := dockerByID[meta.ID]
		lastError := s.store.ReadError(name)
		result = append(result, buildInstance(meta, dc, activeNames[name], lastError))
	}
	return result, nil
}

func (s *service) Start(ctx context.Context, name string, opts StartOptions) (*Instance, error) {
	meta, err := s.store.Read(name)
	if err != nil {
		return nil, fmt.Errorf("instance %q: %w", name, ErrNotFound)
	}

	s.mu.Lock()
	if _, already := s.active[name]; already {
		s.mu.Unlock()
		return nil, fmt.Errorf("instance %q is already starting: %w", name, ErrFailedPrecondition)
	}

	now := time.Now()
	logPath, err := s.store.NewLogPath(name, now, "up")
	if err != nil {
		s.mu.Unlock()
		return nil, err
	}
	lw, err := NewLogWriter(logPath)
	if err != nil {
		s.mu.Unlock()
		return nil, err
	}

	s.active[name] = &activeEntry{writer: lw}
	s.mu.Unlock()

	_ = s.store.ClearError(name)

	s.wg.Add(1)
	go s.runUp(name, meta, lw, opts)

	return &Instance{
		ID: meta.ID, Name: meta.Name, CreatedAt: meta.CreatedAt,
		Status: StatusStarting,
	}, nil
}

func (s *service) Stop(ctx context.Context, name string) (*Instance, error) {
	meta, err := s.store.Read(name)
	if err != nil {
		return nil, fmt.Errorf("instance %q: %w", name, ErrNotFound)
	}

	dc, err := dockerutils.FindByLabel(ctx, s.docker, "norn.id", meta.ID)
	if err != nil {
		return nil, fmt.Errorf("docker: %w", err)
	}
	if dc == nil || dc.State != "running" {
		return nil, fmt.Errorf("instance %q is not running: %w", name, ErrFailedPrecondition)
	}

	if err := dockerutils.Stop(ctx, s.docker, dc.ID); err != nil {
		return nil, err
	}

	return &Instance{
		ID: meta.ID, Name: meta.Name, CreatedAt: meta.CreatedAt,
		DockerID: dc.ID, Status: StatusStopped,
	}, nil
}

func (s *service) Delete(ctx context.Context, name string) error {
	s.mu.Lock()
	_, starting := s.active[name]
	s.mu.Unlock()
	if starting {
		return fmt.Errorf("instance %q is starting: %w", name, ErrFailedPrecondition)
	}

	meta, err := s.store.Read(name)
	if err != nil {
		return fmt.Errorf("instance %q: %w", name, ErrNotFound)
	}

	dc, err := dockerutils.FindByLabel(ctx, s.docker, "norn.id", meta.ID)
	if err != nil {
		return fmt.Errorf("docker: %w", err)
	}
	if dc != nil {
		if err := dockerutils.Remove(ctx, s.docker, dc.ID); err != nil {
			return err
		}
	}

	return s.store.Remove(name)
}

func (s *service) WatchLogs(ctx context.Context, name string, logID string) (<-chan LogEntry, error) {
	if !s.store.Exists(name) {
		return nil, fmt.Errorf("instance %q: %w", name, ErrNotFound)
	}

	var logPath string
	if logID == "" {
		logPath = s.store.LastLogPath(name)
	} else {
		logPath = s.store.LogPathByID(name, logID)
	}
	if logPath == "" {
		return nil, fmt.Errorf("instance %q has no logs: %w", name, ErrNotFound)
	}

	// done channel only for up-logs of a currently starting instance
	s.mu.Lock()
	ae, starting := s.active[name]
	s.mu.Unlock()

	var done <-chan struct{}
	if starting && logID == "" {
		done = ae.writer.Done()
	}

	return WatchLogs(ctx, logPath, done), nil
}

func (s *service) ListLogs(name string) ([]storage.LogFileInfo, error) {
	if !s.store.Exists(name) {
		return nil, fmt.Errorf("instance %q: %w", name, ErrNotFound)
	}
	return s.store.ListLogFiles(name)
}

func (s *service) Shutdown() {
	s.cancel()
	s.wg.Wait()
}

// --- private ---

func (s *service) runUp(name string, meta *storage.InstanceMeta, lw *LogWriter, opts StartOptions) {
	defer s.wg.Done()

	instanceDir := s.home.InstanceDir(name)
	runDir := filepath.Join(instanceDir, "run")

	// Ensure run/ directory exists for daemon socket + metadata
	_ = os.MkdirAll(runDir, 0o755)

	// Resolve norn binary path for mount
	nornBin, _ := os.Executable()

	mp := s.dcMountPath
	upOpts := devcontainer.UpOptions{
		Global: s.opts,
		Labels: map[string]string{
			"norn.id":   meta.ID,
			"norn.name": meta.Name,
		},
		Env: map[string]string{
			"DOTS_PATH":         filepath.Join(s.home.BaseDir(), "shared", "dotfiles"),
			"INSTANCE_MNT_PATH": filepath.Join(instanceDir, "mnt"),
		},
		Mounts: []string{
			fmt.Sprintf("type=bind,source=%s,target=%s", nornBin, filepath.Join(mp, "bin", "norn")),
			fmt.Sprintf("type=bind,source=%s,target=%s", filepath.Join(s.home.BaseDir(), "shared", "dotfiles"), filepath.Join(mp, "dotfiles")),
			fmt.Sprintf("type=bind,source=%s,target=%s", instanceDir, filepath.Join(mp, "instance")),
			fmt.Sprintf("type=bind,source=%s,target=%s", runDir, filepath.Join(mp, "run")),
		},
		RemoveExisting: opts.RemoveExisting,
	}

	_, err := s.dc.Up(s.bgCtx, upOpts, lw.Stdout(), lw.Stderr())
	if err != nil {
		zap.L().Error("devcontainer up failed", zap.String("instance", name), zap.Error(err))
		fmt.Fprintln(lw.Stderr(), "ERROR:", err)
		_ = s.store.WriteError(name, err.Error())
		lw.Close()
		s.mu.Lock()
		delete(s.active, name)
		s.mu.Unlock()
		return
	}

	// Launch daemon via nohup devcontainer exec on the host, redirect to log file.
	daemonBin := filepath.Join(mp, "bin", "norn")
	socketPath := filepath.Join(mp, "run", "daemon.sock")
	runDirInContainer := filepath.Join(mp, "run")

	now := time.Now()
	daemonLogPath, logErr := s.store.NewLogPath(name, now, "daemon")
	if logErr != nil {
		zap.L().Error("failed to create daemon log path", zap.String("instance", name), zap.Error(logErr))
		fmt.Fprintln(lw.Stderr(), "ERROR: daemon log path:", logErr)
	} else {
		daemonArgs := s.opts.BaseArgs()
		daemonArgs = append(daemonArgs, "--id-label", "norn.name="+name)
		daemonArgs = append(daemonArgs, daemonBin, "run", "daemon", "--socket", socketPath, "--run-dir", runDirInContainer)

		cmd := exec.Command("nohup", append([]string{"devcontainer", "exec"}, daemonArgs...)...)
		logFile, fErr := os.OpenFile(daemonLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if fErr != nil {
			zap.L().Error("failed to open daemon log file", zap.String("instance", name), zap.Error(fErr))
			fmt.Fprintln(lw.Stderr(), "ERROR: daemon log file:", fErr)
		} else {
			cmd.Stdout = logFile
			cmd.Stderr = logFile
			if startErr := cmd.Start(); startErr != nil {
				zap.L().Warn("daemon launch failed", zap.String("instance", name), zap.Error(startErr))
				fmt.Fprintln(lw.Stderr(), "WARN: daemon launch:", startErr)
			} else {
				zap.L().Info("daemon started", zap.String("instance", name), zap.Int("pid", cmd.Process.Pid))
				// Monitor daemon in background: log exit and record error.
				go func() {
					waitErr := cmd.Wait()
					_ = logFile.Close()
					if waitErr != nil {
						zap.L().Error("daemon exited with error", zap.String("instance", name), zap.Error(waitErr))
						_ = s.store.WriteError(name, "daemon exited: "+waitErr.Error())
					} else {
						zap.L().Warn("daemon exited", zap.String("instance", name))
					}
				}()
			}
		}
	}

	lw.Close()

	s.mu.Lock()
	delete(s.active, name)
	s.mu.Unlock()
}

func buildInstance(meta *storage.InstanceMeta, dc *types.Container, isStarting bool, lastError string) *Instance {
	var dockerID, dockerState string
	if dc != nil {
		dockerID = dc.ID
		dockerState = dc.State
	}
	return &Instance{
		ID:           meta.ID,
		Name:         meta.Name,
		CreatedAt:    meta.CreatedAt,
		DockerID:     dockerID,
		Status:       resolveStatus(isStarting, dockerState, lastError),
		ErrorMessage: lastError,
	}
}

func resolveStatus(isStarting bool, dockerState, lastError string) ContainerStatus {
	if isStarting {
		return StatusStarting
	}
	// Check lastError before docker state: if daemon died inside a
	// running container, the instance is in an error state.
	if lastError != "" {
		return StatusError
	}
	if dockerState == "running" {
		return StatusRunning
	}
	return StatusStopped
}
