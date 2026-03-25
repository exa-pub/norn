package container

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/exa-pub/norn/internal/entity"
	"github.com/exa-pub/norn/internal/pkg/devcontainer"
	"github.com/exa-pub/norn/internal/pkg/docker"
)

const (
	labelName    = "norn.name"
	labelID      = "norn.id"
	labelStorage = "norn.storage"
)

var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]{0,62}$`)

var (
	ErrNotFound           = errors.New("not found")
	ErrAlreadyExists      = errors.New("already exists")
	ErrInvalidName        = errors.New("invalid name")
	ErrFailedPrecondition = errors.New("failed precondition")
)

type containerMeta struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	DockerID     string    `json:"docker_id,omitempty"`
	Status       string    `json:"status,omitempty"`       // "starting", "running", "stopped", "error"
	ErrorMessage string    `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// Config holds norn-specific settings for the manager.
type Config struct {
	WorkspaceFolder string
	ConfigPath      string
	StorageDir      string
}

// StartOptions configures a container start.
type StartOptions struct {
	RemoveExisting bool
}

// Manager orchestrates container lifecycle.
type Manager interface {
	Create(ctx context.Context, name string) (*entity.Container, error)
	Get(ctx context.Context, name string) (*entity.Container, error)
	List(ctx context.Context) ([]*entity.Container, error)
	Start(ctx context.Context, name string, opts StartOptions) (*entity.Container, error)
	Stop(ctx context.Context, name string) (*entity.Container, error)
	Delete(ctx context.Context, name string) error
	LogBusFor(name string) (*LogBus, bool)
	// Shutdown cancels all background operations and waits for them to finish.
	Shutdown()
}

type manager struct {
	dc     devcontainer.Client
	docker docker.Client
	cfg    Config

	mu     sync.RWMutex
	active map[string]*LogBus

	cancel context.CancelFunc
	bgCtx  context.Context
	wg     sync.WaitGroup
}

func NewManager(dc devcontainer.Client, docker docker.Client, cfg Config) Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &manager{
		dc:     dc,
		docker: docker,
		cfg:    cfg,
		active: make(map[string]*LogBus),
		bgCtx:  ctx,
		cancel: cancel,
	}
}

// Create validates name, writes meta.json. Does NOT start the container.
func (m *manager) Create(ctx context.Context, name string) (*entity.Container, error) {
	if !validName.MatchString(name) {
		return nil, fmt.Errorf("%w: %q (use lowercase letters, digits and hyphens)", ErrInvalidName, name)
	}

	metaPath := filepath.Join(m.instanceDir(name), "meta.json")
	if _, err := os.Stat(metaPath); err == nil {
		return nil, fmt.Errorf("container %q: %w", name, ErrAlreadyExists)
	}

	now := time.Now()
	meta := containerMeta{
		ID:        uuid.New().String(),
		Name:      name,
		Status:    string(entity.ContainerStatusStopped),
		CreatedAt: now,
	}

	if err := m.ensureDirs(name); err != nil {
		return nil, err
	}
	if err := m.writeMeta(name, meta); err != nil {
		return nil, err
	}

	return &entity.Container{
		ID:        meta.ID,
		Name:      name,
		Status:    entity.ContainerStatusStopped,
		CreatedAt: now,
	}, nil
}

// Get returns a single container by name.
func (m *manager) Get(ctx context.Context, name string) (*entity.Container, error) {
	meta, err := m.readMeta(name)
	if err != nil {
		return nil, fmt.Errorf("container %q: %w", name, ErrNotFound)
	}

	m.mu.RLock()
	_, starting := m.active[name]
	m.mu.RUnlock()

	dc, err := m.docker.FindByLabel(ctx, labelName, name)
	if err != nil {
		return nil, fmt.Errorf("docker find: %w", err)
	}

	if dc == nil {
		return &entity.Container{
			ID:           meta.ID,
			Name:         meta.Name,
			DockerID:     meta.DockerID,
			Status:       m.resolveStatus(meta, starting),
			ErrorMessage: meta.ErrorMessage,
			CreatedAt:    meta.CreatedAt,
		}, nil
	}

	return m.toEntity(dc, meta, starting), nil
}

// List returns all norn-managed containers by scanning storage.
func (m *manager) List(ctx context.Context) ([]*entity.Container, error) {
	instancesDir := filepath.Join(m.cfg.StorageDir, "instances")
	entries, err := os.ReadDir(instancesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read instances dir: %w", err)
	}

	m.mu.RLock()
	activeNames := make(map[string]bool, len(m.active))
	for name := range m.active {
		activeNames[name] = true
	}
	m.mu.RUnlock()

	dcs, err := m.docker.ListByLabel(ctx, labelName)
	if err != nil {
		return nil, fmt.Errorf("docker list: %w", err)
	}
	dockerByName := make(map[string]*docker.Container, len(dcs))
	for _, dc := range dcs {
		n := dc.Labels[labelName]
		if n != "" {
			dockerByName[n] = dc
		}
	}

	var result []*entity.Container
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		meta, err := m.readMeta(name)
		if err != nil {
			continue
		}

		dc := dockerByName[name]
		if dc != nil {
			result = append(result, m.toEntity(dc, meta, activeNames[name]))
		} else {
			result = append(result, &entity.Container{
				ID:           meta.ID,
				Name:         meta.Name,
				DockerID:     meta.DockerID,
				Status:       m.resolveStatus(meta, activeNames[name]),
				ErrorMessage: meta.ErrorMessage,
				CreatedAt:    meta.CreatedAt,
			})
		}
	}

	return result, nil
}

// Start runs devcontainer up for a container.
func (m *manager) Start(ctx context.Context, name string, opts StartOptions) (*entity.Container, error) {
	meta, err := m.readMeta(name)
	if err != nil {
		return nil, fmt.Errorf("container %q: %w", name, ErrNotFound)
	}

	m.mu.RLock()
	_, already := m.active[name]
	m.mu.RUnlock()
	if already {
		return nil, fmt.Errorf("container %q is already starting: %w", name, ErrFailedPrecondition)
	}

	// Clear previous error state.
	meta.Status = string(entity.ContainerStatusStarting)
	meta.ErrorMessage = ""
	_ = m.writeMeta(name, *meta)

	now := time.Now()
	bus, err := m.openLogBus(name, now)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.active[name] = bus
	m.mu.Unlock()

	m.wg.Add(1)
	go m.runUp(name, meta, bus, opts)

	return &entity.Container{
		ID:        meta.ID,
		Name:      meta.Name,
		Status:    entity.ContainerStatusStarting,
		CreatedAt: meta.CreatedAt,
	}, nil
}

// Stop stops a running container.
func (m *manager) Stop(ctx context.Context, name string) (*entity.Container, error) {
	meta, err := m.readMeta(name)
	if err != nil {
		return nil, fmt.Errorf("container %q: %w", name, ErrNotFound)
	}

	dc, err := m.docker.FindByLabel(ctx, labelName, name)
	if err != nil {
		return nil, fmt.Errorf("docker find: %w", err)
	}
	if dc == nil {
		return nil, fmt.Errorf("container %q: %w", name, ErrNotFound)
	}
	if dc.State != "running" {
		return nil, fmt.Errorf("container %q is not running (state: %s): %w", name, dc.State, ErrFailedPrecondition)
	}

	if err := m.docker.Stop(ctx, dc.ID); err != nil {
		return nil, err
	}

	meta.Status = string(entity.ContainerStatusStopped)
	_ = m.writeMeta(name, *meta)

	return &entity.Container{
		ID:        meta.ID,
		Name:      meta.Name,
		DockerID:  dc.ID,
		Status:    entity.ContainerStatusStopped,
		CreatedAt: meta.CreatedAt,
	}, nil
}

// Delete removes the container and its instance directory.
func (m *manager) Delete(ctx context.Context, name string) error {
	m.mu.RLock()
	_, starting := m.active[name]
	m.mu.RUnlock()
	if starting {
		return fmt.Errorf("container %q is starting, wait for it to finish: %w", name, ErrFailedPrecondition)
	}

	dc, err := m.docker.FindByLabel(ctx, labelName, name)
	if err != nil {
		return fmt.Errorf("docker find: %w", err)
	}
	if dc != nil {
		if err := m.docker.Remove(ctx, dc.ID); err != nil {
			return err
		}
	}

	if err := os.RemoveAll(m.instanceDir(name)); err != nil {
		return fmt.Errorf("remove instance dir: %w", err)
	}
	return nil
}

// LogBusFor returns the active LogBus for a container that is currently starting.
func (m *manager) LogBusFor(name string) (*LogBus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	bus, ok := m.active[name]
	return bus, ok
}

// Shutdown cancels background operations and waits for goroutines to finish.
func (m *manager) Shutdown() {
	m.cancel()
	m.wg.Wait()
}

// --- private ---

func (m *manager) instanceDir(name string) string {
	return filepath.Join(m.cfg.StorageDir, "instances", name)
}

func (m *manager) writeMeta(name string, meta containerMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}
	path := filepath.Join(m.instanceDir(name), "meta.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write meta: %w", err)
	}
	return nil
}

func (m *manager) readMeta(name string) (*containerMeta, error) {
	path := filepath.Join(m.instanceDir(name), "meta.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var meta containerMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("unmarshal meta: %w", err)
	}
	return &meta, nil
}

func (m *manager) runUp(name string, meta *containerMeta, bus *LogBus, opts StartOptions) {
	defer m.wg.Done()

	upOpts := devcontainer.UpOptions{
		WorkspaceFolder: m.cfg.WorkspaceFolder,
		ConfigPath:      m.cfg.ConfigPath,
		Labels: map[string]string{
			labelName:    name,
			labelID:      meta.ID,
			labelStorage: m.instanceDir(name),
		},
		Env: map[string]string{
			"DOTS_PATH":         filepath.Join(m.cfg.StorageDir, "dots"),
			"INSTANCE_MNT_PATH": filepath.Join(m.instanceDir(name), "mnt"),
			"SESSIONS_PATH":     filepath.Join(m.instanceDir(name), "sessions"),
		},
		RemoveExisting: opts.RemoveExisting,
	}

	dockerID, err := m.dc.Up(m.bgCtx, upOpts, bus.Stdout(), bus.Stderr())

	// Update meta with result.
	if err != nil {
		fmt.Fprintln(bus.Stderr(), "ERROR:", err)
		meta.Status = string(entity.ContainerStatusError)
		meta.ErrorMessage = err.Error()
	} else {
		meta.DockerID = dockerID
		meta.Status = string(entity.ContainerStatusRunning)
		meta.ErrorMessage = ""
	}
	_ = m.writeMeta(name, *meta)

	bus.Close()

	m.mu.Lock()
	delete(m.active, name)
	m.mu.Unlock()
}

func (m *manager) ensureDirs(name string) error {
	dirs := []string{
		filepath.Join(m.cfg.StorageDir, "dots"),
		filepath.Join(m.instanceDir(name), "mnt"),
		filepath.Join(m.instanceDir(name), "sessions"),
		filepath.Join(m.instanceDir(name), "logs"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return nil
}

func (m *manager) openLogBus(name string, now time.Time) (*LogBus, error) {
	logDir := filepath.Join(m.instanceDir(name), "logs", now.Format("2006-01-02"))
	bus, err := newLogBus(logDir, now.Format("15-04-05"))
	if err != nil {
		return nil, fmt.Errorf("open log bus: %w", err)
	}
	return bus, nil
}

func (m *manager) toEntity(dc *docker.Container, meta *containerMeta, isStarting bool) *entity.Container {
	return &entity.Container{
		ID:           meta.ID,
		Name:         meta.Name,
		DockerID:     dc.ID,
		Status:       dockerStateToStatus(dc.State, isStarting),
		ErrorMessage: meta.ErrorMessage,
		CreatedAt:    meta.CreatedAt,
	}
}

func (m *manager) resolveStatus(meta *containerMeta, isStarting bool) entity.ContainerStatus {
	if isStarting {
		return entity.ContainerStatusStarting
	}
	switch entity.ContainerStatus(meta.Status) {
	case entity.ContainerStatusError:
		return entity.ContainerStatusError
	case entity.ContainerStatusRunning:
		// Docker container might have stopped since we last wrote meta.
		// Caller should prefer Docker state when available.
		return entity.ContainerStatusStopped
	default:
		return entity.ContainerStatusStopped
	}
}

func dockerStateToStatus(state string, isStarting bool) entity.ContainerStatus {
	if isStarting {
		return entity.ContainerStatusStarting
	}
	switch state {
	case "running":
		return entity.ContainerStatusRunning
	case "exited", "dead":
		return entity.ContainerStatusStopped
	default:
		return entity.ContainerStatusStopped
	}
}
