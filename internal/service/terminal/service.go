package terminal

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"github.com/exa-pub/norn/internal/entity"
	"github.com/exa-pub/norn/internal/service/tty"
)

type Service interface {
	Create(ctx context.Context, instanceName, name string) (*entity.TerminalSession, error)
	Get(id string) (*entity.TerminalSession, error)
	List(instanceName string) []*entity.TerminalSession
	Close(id string) error
}

type entry struct {
	terminal *entity.TerminalSession
}

type service struct {
	ttyMgr tty.Manager

	mu       sync.Mutex
	sessions map[string]*entry
}

func NewService(ttyMgr tty.Manager) Service {
	return &service{
		ttyMgr:   ttyMgr,
		sessions: make(map[string]*entry),
	}
}

func (s *service) Create(ctx context.Context, instanceName, name string) (*entity.TerminalSession, error) {
	termID := uuid.New().String()

	ttySess, err := s.ttyMgr.Create(ctx, instanceName, []string{"/bin/bash"}, func() {
		s.mu.Lock()
		delete(s.sessions, termID)
		s.mu.Unlock()
	})
	if err != nil {
		return nil, fmt.Errorf("create tty: %w", err)
	}

	term := &entity.TerminalSession{
		ID:           termID,
		InstanceName: instanceName,
		Name:         name,
		TTYID:        ttySess.ID,
	}

	s.mu.Lock()
	s.sessions[termID] = &entry{terminal: term}
	s.mu.Unlock()

	return term, nil
}

func (s *service) Get(id string) (*entity.TerminalSession, error) {
	s.mu.Lock()
	e, ok := s.sessions[id]
	s.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("terminal %q: %w", id, entity.ErrNotFound)
	}
	return e.terminal, nil
}

func (s *service) List(instanceName string) []*entity.TerminalSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []*entity.TerminalSession
	for _, e := range s.sessions {
		if e.terminal.InstanceName == instanceName {
			result = append(result, e.terminal)
		}
	}
	return result
}

func (s *service) Close(id string) error {
	s.mu.Lock()
	e, ok := s.sessions[id]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("terminal %q: %w", id, entity.ErrNotFound)
	}
	delete(s.sessions, id)
	s.mu.Unlock()

	return s.ttyMgr.Close(e.terminal.TTYID)
}
