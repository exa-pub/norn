package agent

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/exa-pub/norn/internal/entity"
	"github.com/exa-pub/norn/internal/service/storage"
)

// Service manages Agent Session lifecycle (CRUD).
// Launch/Stop will be added in Phase 4 when TTY is available.
type Service interface {
	CreateSession(ctx context.Context, instanceName, name string) (*entity.AgentSession, error)
	GetSession(ctx context.Context, instanceName, sessionID string) (*entity.AgentSession, error)
	ListSessions(ctx context.Context, instanceName string) ([]*entity.AgentSession, error)
	DeleteSession(ctx context.Context, instanceName, sessionID string) error
}

type service struct {
	store    storage.AgentStore
	instStore storage.InstanceStore
}

func NewService(store storage.AgentStore, instStore storage.InstanceStore) Service {
	return &service{store: store, instStore: instStore}
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
	return &entity.AgentSession{ID: meta.ID, Name: meta.Name, Running: false}, nil
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
		result = append(result, &entity.AgentSession{ID: m.ID, Name: m.Name, Running: false})
	}
	return result, nil
}

func (s *service) DeleteSession(ctx context.Context, instanceName, sessionID string) error {
	if _, err := s.store.ReadAgent(instanceName, sessionID); err != nil {
		return fmt.Errorf("agent session %q: %w", sessionID, entity.ErrNotFound)
	}
	return s.store.RemoveAgent(instanceName, sessionID)
}
