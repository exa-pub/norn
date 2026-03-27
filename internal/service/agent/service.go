// Package agent manages persistent Agent Sessions (Claude Code conversations).
//
// On server restart, the running map is empty. Previously launched agents
// may still be running as Docker exec processes, but are considered stopped
// from Norn's perspective. A new Launch will create a fresh exec;
// the orphaned exec will terminate when its stdin is closed.
package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/docker/docker/client"
	"github.com/google/uuid"

	"github.com/exa-pub/norn/internal/entity"
	"github.com/exa-pub/norn/internal/service/storage"
	"github.com/exa-pub/norn/internal/service/tty"
	"github.com/exa-pub/norn/pkg/dockerutils"
)

type Service interface {
	CreateSession(ctx context.Context, instanceName, name string) (*entity.AgentSession, error)
	GetSession(ctx context.Context, instanceName, sessionID string) (*entity.AgentSession, error)
	ListSessions(ctx context.Context, instanceName string) ([]*entity.AgentSession, error)
	DeleteSession(ctx context.Context, instanceName, sessionID string) error
	UpdateSessionName(ctx context.Context, instanceName, sessionID, name string) (*entity.AgentSession, error)
	Launch(ctx context.Context, instanceName, sessionID, prompt string) (*entity.AgentSession, error)
	Stop(ctx context.Context, instanceName, sessionID string) (*entity.AgentSession, error)
}

type service struct {
	store     storage.AgentStore
	instStore storage.InstanceStore
	ttyMgr    tty.Manager
	docker    *client.Client

	mu      sync.Mutex
	running map[string]string // sessionUUID → tty session ID
}

func NewService(store storage.AgentStore, instStore storage.InstanceStore, ttyMgr tty.Manager, dk *client.Client) Service {
	return &service{
		store:     store,
		instStore: instStore,
		ttyMgr:    ttyMgr,
		docker:    dk,
		running:   make(map[string]string),
	}
}

func (s *service) CreateSession(ctx context.Context, instanceName, name string) (*entity.AgentSession, error) {
	if !s.instStore.Exists(instanceName) {
		return nil, fmt.Errorf("instance %q: %w", instanceName, entity.ErrNotFound)
	}

	sessionID := uuid.New().String()
	meta := storage.AgentMeta{ID: sessionID, Name: name}
	if err := s.store.CreateAgent(instanceName, meta); err != nil {
		return nil, err
	}

	return s.toEntity(sessionID, name), nil
}

func (s *service) GetSession(ctx context.Context, instanceName, sessionID string) (*entity.AgentSession, error) {
	meta, err := s.store.ReadAgent(instanceName, sessionID)
	if err != nil {
		return nil, fmt.Errorf("agent session %q: %w", sessionID, entity.ErrNotFound)
	}
	return s.toEntity(meta.ID, meta.Name), nil
}

func (s *service) ListSessions(ctx context.Context, instanceName string) ([]*entity.AgentSession, error) {
	if !s.instStore.Exists(instanceName) {
		return nil, fmt.Errorf("instance %q: %w", instanceName, entity.ErrNotFound)
	}

	metas, err := s.store.ListAgents(instanceName)
	if err != nil {
		return nil, err
	}

	result := make([]*entity.AgentSession, 0, len(metas))
	for _, m := range metas {
		result = append(result, s.toEntity(m.ID, m.Name))
	}
	return result, nil
}

func (s *service) DeleteSession(ctx context.Context, instanceName, sessionID string) error {
	s.mu.Lock()
	_, running := s.running[sessionID]
	s.mu.Unlock()
	if running {
		return fmt.Errorf("agent session %q is running, stop it first: %w", sessionID, entity.ErrFailedPrecondition)
	}

	if _, err := s.store.ReadAgent(instanceName, sessionID); err != nil {
		return fmt.Errorf("agent session %q: %w", sessionID, entity.ErrNotFound)
	}
	return s.store.RemoveAgent(instanceName, sessionID)
}

func (s *service) UpdateSessionName(ctx context.Context, instanceName, sessionID, name string) (*entity.AgentSession, error) {
	if err := s.store.UpdateAgentName(instanceName, sessionID, name); err != nil {
		return nil, fmt.Errorf("agent session %q: %w", sessionID, entity.ErrNotFound)
	}
	return s.toEntity(sessionID, name), nil
}

func (s *service) Launch(ctx context.Context, instanceName, sessionID, prompt string) (*entity.AgentSession, error) {
	meta, err := s.store.ReadAgent(instanceName, sessionID)
	if err != nil {
		return nil, fmt.Errorf("agent session %q: %w", sessionID, entity.ErrNotFound)
	}

	// Verify container is running before creating TTY.
	instMeta, err := s.instStore.Read(instanceName)
	if err != nil {
		return nil, fmt.Errorf("instance %q: %w", instanceName, entity.ErrNotFound)
	}
	dc, err := dockerutils.FindByLabel(ctx, s.docker, "norn.id", instMeta.ID)
	if err != nil {
		return nil, fmt.Errorf("docker: %w", err)
	}
	if dc == nil || dc.State != "running" {
		return nil, fmt.Errorf("instance %q has no running container: %w", instanceName, entity.ErrFailedPrecondition)
	}

	s.mu.Lock()
	if _, already := s.running[sessionID]; already {
		s.mu.Unlock()
		return nil, fmt.Errorf("agent session %q already running: %w", sessionID, entity.ErrFailedPrecondition)
	}
	s.mu.Unlock()

	cmd := []string{
		"claude",
		"--resume", sessionID,
		"--dangerously-skip-permissions",
	}
	if prompt != "" {
		cmd = append(cmd, "-p", prompt)
	}

	var ttyID string
	ttySess, err := s.ttyMgr.Create(ctx, instanceName, cmd, func() {
		s.mu.Lock()
		if s.running[sessionID] == ttyID {
			delete(s.running, sessionID)
		}
		s.mu.Unlock()
	})
	if err != nil {
		return nil, fmt.Errorf("create tty for agent: %w", err)
	}
	ttyID = ttySess.ID

	s.mu.Lock()
	s.running[sessionID] = ttyID
	s.mu.Unlock()

	return &entity.AgentSession{
		ID: meta.ID, Name: meta.Name,
		Running: true, TTYID: ttyID,
	}, nil
}

func (s *service) Stop(ctx context.Context, instanceName, sessionID string) (*entity.AgentSession, error) {
	meta, err := s.store.ReadAgent(instanceName, sessionID)
	if err != nil {
		return nil, fmt.Errorf("agent session %q: %w", sessionID, entity.ErrNotFound)
	}

	s.mu.Lock()
	ttyID, running := s.running[sessionID]
	if running {
		delete(s.running, sessionID)
	}
	s.mu.Unlock()

	if running {
		_ = s.ttyMgr.Close(ttyID)
	}

	return &entity.AgentSession{ID: meta.ID, Name: meta.Name, Running: false}, nil
}

// --- private ---

// toEntity builds an AgentSession with current running/ttyID state from the in-memory map.
func (s *service) toEntity(id, name string) *entity.AgentSession {
	s.mu.Lock()
	ttyID, running := s.running[id]
	s.mu.Unlock()

	return &entity.AgentSession{
		ID: id, Name: name,
		Running: running, TTYID: ttyID,
	}
}

