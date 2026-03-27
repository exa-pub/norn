package instance

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/google/uuid"

	"github.com/exa-pub/norn/internal/entity"
	"github.com/exa-pub/norn/internal/pkg/devcontainer"
	"github.com/exa-pub/norn/internal/service/storage"
	"github.com/exa-pub/norn/pkg/dockerutils"
)

var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]{0,62}$`)

type StartOptions struct {
	RemoveExisting bool
}

type Config struct {
	WorkspaceFolder string
	ConfigPath      string
}

type Service interface {
	Create(ctx context.Context, name string) (*entity.Instance, error)
	Get(ctx context.Context, name string) (*entity.Instance, error)
	List(ctx context.Context) ([]*entity.Instance, error)
	Start(ctx context.Context, name string, opts StartOptions) (*entity.Instance, error)
	Stop(ctx context.Context, name string) (*entity.Instance, error)
	Delete(ctx context.Context, name string) error
	WatchLogs(ctx context.Context, name string) (<-chan LogEntry, error)
	Shutdown()
}

type activeEntry struct {
	writer *LogWriter
}

type service struct {
	store  storage.InstanceStore
	home   storage.Home
	dc     devcontainer.Client
	docker *client.Client
	cfg    Config

	mu     sync.Mutex
	active map[string]*activeEntry

	cancel context.CancelFunc
	bgCtx  context.Context
	wg     sync.WaitGroup
}

func NewService(store storage.InstanceStore, home storage.Home, dc devcontainer.Client, dk *client.Client, cfg Config) Service {
	ctx, cancel := context.WithCancel(context.Background())
	return &service{
		store:  store,
		home:   home,
		dc:     dc,
		docker: dk,
		cfg:    cfg,
		active: make(map[string]*activeEntry),
		bgCtx:  ctx,
		cancel: cancel,
	}
}

func (s *service) Create(ctx context.Context, name string) (*entity.Instance, error) {
	if !validName.MatchString(name) {
		return nil, fmt.Errorf("%w: %q", entity.ErrInvalidName, name)
	}
	if s.store.Exists(name) {
		return nil, fmt.Errorf("instance %q: %w", name, entity.ErrAlreadyExists)
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

	return &entity.Instance{
		ID: meta.ID, Name: name, CreatedAt: now,
		Status: entity.StatusStopped,
	}, nil
}

func (s *service) Get(ctx context.Context, name string) (*entity.Instance, error) {
	meta, err := s.store.Read(name)
	if err != nil {
		return nil, fmt.Errorf("instance %q: %w", name, entity.ErrNotFound)
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

func (s *service) List(ctx context.Context) ([]*entity.Instance, error) {
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

	var result []*entity.Instance
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

func (s *service) Start(ctx context.Context, name string, opts StartOptions) (*entity.Instance, error) {
	meta, err := s.store.Read(name)
	if err != nil {
		return nil, fmt.Errorf("instance %q: %w", name, entity.ErrNotFound)
	}

	s.mu.Lock()
	if _, already := s.active[name]; already {
		s.mu.Unlock()
		return nil, fmt.Errorf("instance %q is already starting: %w", name, entity.ErrFailedPrecondition)
	}

	now := time.Now()
	logPath, err := s.store.NewLogPath(name, now)
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

	return &entity.Instance{
		ID: meta.ID, Name: meta.Name, CreatedAt: meta.CreatedAt,
		Status: entity.StatusStarting,
	}, nil
}

func (s *service) Stop(ctx context.Context, name string) (*entity.Instance, error) {
	meta, err := s.store.Read(name)
	if err != nil {
		return nil, fmt.Errorf("instance %q: %w", name, entity.ErrNotFound)
	}

	dc, err := dockerutils.FindByLabel(ctx, s.docker, "norn.id", meta.ID)
	if err != nil {
		return nil, fmt.Errorf("docker: %w", err)
	}
	if dc == nil || dc.State != "running" {
		return nil, fmt.Errorf("instance %q is not running: %w", name, entity.ErrFailedPrecondition)
	}

	if err := dockerutils.Stop(ctx, s.docker, dc.ID); err != nil {
		return nil, err
	}

	return &entity.Instance{
		ID: meta.ID, Name: meta.Name, CreatedAt: meta.CreatedAt,
		DockerID: dc.ID, Status: entity.StatusStopped,
	}, nil
}

func (s *service) Delete(ctx context.Context, name string) error {
	s.mu.Lock()
	_, starting := s.active[name]
	s.mu.Unlock()
	if starting {
		return fmt.Errorf("instance %q is starting: %w", name, entity.ErrFailedPrecondition)
	}

	meta, err := s.store.Read(name)
	if err != nil {
		return fmt.Errorf("instance %q: %w", name, entity.ErrNotFound)
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

func (s *service) WatchLogs(ctx context.Context, name string) (<-chan LogEntry, error) {
	if !s.store.Exists(name) {
		return nil, fmt.Errorf("instance %q: %w", name, entity.ErrNotFound)
	}

	s.mu.Lock()
	ae, starting := s.active[name]
	s.mu.Unlock()

	logPath := s.store.LastLogPath(name)
	if logPath == "" {
		return nil, fmt.Errorf("instance %q has no logs: %w", name, entity.ErrNotFound)
	}

	var done <-chan struct{}
	if starting {
		done = ae.writer.Done()
	}

	return WatchLogs(ctx, logPath, done), nil
}

func (s *service) Shutdown() {
	s.cancel()
	s.wg.Wait()
}

// --- private ---

func (s *service) runUp(name string, meta *storage.InstanceMeta, lw *LogWriter, opts StartOptions) {
	defer s.wg.Done()

	instanceDir := s.home.InstanceDir(name)
	upOpts := devcontainer.UpOptions{
		WorkspaceFolder: s.cfg.WorkspaceFolder,
		ConfigPath:      s.cfg.ConfigPath,
		Labels: map[string]string{
			"norn.id":   meta.ID,
			"norn.name": meta.Name,
		},
		Env: map[string]string{
			"DOTS_PATH":         filepath.Join(s.home.BaseDir(), "shared", "dotfiles"),
			"INSTANCE_MNT_PATH": filepath.Join(instanceDir, "mnt"),
		},
		RemoveExisting: opts.RemoveExisting,
	}

	_, err := s.dc.Up(s.bgCtx, upOpts, lw.Stdout(), lw.Stderr())
	if err != nil {
		fmt.Fprintln(lw.Stderr(), "ERROR:", err)
		_ = s.store.WriteError(name, err.Error())
	}

	lw.Close()

	s.mu.Lock()
	delete(s.active, name)
	s.mu.Unlock()
}

func buildInstance(meta *storage.InstanceMeta, dc *types.Container, isStarting bool, lastError string) *entity.Instance {
	var dockerID, dockerState string
	if dc != nil {
		dockerID = dc.ID
		dockerState = dc.State
	}
	return &entity.Instance{
		ID:           meta.ID,
		Name:         meta.Name,
		CreatedAt:    meta.CreatedAt,
		DockerID:     dockerID,
		Status:       resolveStatus(isStarting, dockerState, lastError),
		ErrorMessage: lastError,
	}
}

func resolveStatus(isStarting bool, dockerState, lastError string) entity.ContainerStatus {
	if isStarting {
		return entity.StatusStarting
	}
	if dockerState == "running" {
		return entity.StatusRunning
	}
	if lastError != "" {
		return entity.StatusError
	}
	return entity.StatusStopped
}
