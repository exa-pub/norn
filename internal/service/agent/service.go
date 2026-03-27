package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/exa-pub/norn/internal/entity"
	"github.com/exa-pub/norn/internal/pkg/docker"
	"github.com/exa-pub/norn/internal/service/storage"
	"github.com/exa-pub/norn/internal/service/tty"
)

type Service interface {
	CreateSession(ctx context.Context, instanceName, name string) (*entity.AgentSession, error)
	GetSession(ctx context.Context, instanceName, sessionID string) (*entity.AgentSession, error)
	ListSessions(ctx context.Context, instanceName string) ([]*entity.AgentSession, error)
	DeleteSession(ctx context.Context, instanceName, sessionID string) error
	Launch(ctx context.Context, instanceName, sessionID, prompt string) (*entity.AgentSession, error)
	Stop(ctx context.Context, instanceName, sessionID string) (*entity.AgentSession, error)
}

type service struct {
	store     storage.AgentStore
	instStore storage.InstanceStore
	ttyMgr    tty.Manager
	docker    docker.Client

	mu      sync.Mutex
	running map[string]string // sessionUUID → tty session ID
}

func NewService(store storage.AgentStore, instStore storage.InstanceStore, ttyMgr tty.Manager, dk docker.Client) Service {
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

	return &entity.AgentSession{ID: sessionID, Name: name, Running: false}, nil
}

func (s *service) GetSession(ctx context.Context, instanceName, sessionID string) (*entity.AgentSession, error) {
	meta, err := s.store.ReadAgent(instanceName, sessionID)
	if err != nil {
		return nil, fmt.Errorf("agent session %q: %w", sessionID, entity.ErrNotFound)
	}

	s.mu.Lock()
	_, running := s.running[sessionID]
	s.mu.Unlock()

	return &entity.AgentSession{ID: meta.ID, Name: meta.Name, Running: running}, nil
}

func (s *service) ListSessions(ctx context.Context, instanceName string) ([]*entity.AgentSession, error) {
	if !s.instStore.Exists(instanceName) {
		return nil, fmt.Errorf("instance %q: %w", instanceName, entity.ErrNotFound)
	}

	metas, err := s.store.ListAgents(instanceName)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]*entity.AgentSession, 0, len(metas))
	for _, m := range metas {
		_, running := s.running[m.ID]
		result = append(result, &entity.AgentSession{ID: m.ID, Name: m.Name, Running: running})
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

func (s *service) Launch(ctx context.Context, instanceName, sessionID, prompt string) (*entity.AgentSession, error) {
	meta, err := s.store.ReadAgent(instanceName, sessionID)
	if err != nil {
		return nil, fmt.Errorf("agent session %q: %w", sessionID, entity.ErrNotFound)
	}

	s.mu.Lock()
	if _, already := s.running[sessionID]; already {
		s.mu.Unlock()
		return nil, fmt.Errorf("agent session %q already running: %w", sessionID, entity.ErrFailedPrecondition)
	}
	s.mu.Unlock()

	// Build claude command.
	cmd := []string{
		"claude",
		"--resume", sessionID,
		"--dangerously-skip-permissions",
	}
	if prompt != "" {
		cmd = append(cmd, "-p", prompt)
	}

	ttySess, err := s.ttyMgr.Create(ctx, instanceName, cmd)
	if err != nil {
		return nil, fmt.Errorf("create tty for agent: %w", err)
	}

	s.mu.Lock()
	s.running[sessionID] = ttySess.ID
	s.mu.Unlock()

	// Monitor TTY exit to update running state.
	go s.watchExit(sessionID, ttySess.ID)

	return &entity.AgentSession{ID: meta.ID, Name: meta.Name, Running: true}, nil
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

func (s *service) watchExit(sessionID, ttyID string) {
	for {
		if _, ok := s.ttyMgr.Get(ttyID); !ok {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	s.mu.Lock()
	if s.running[sessionID] == ttyID {
		delete(s.running, sessionID)
	}
	s.mu.Unlock()
}
