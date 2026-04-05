// Package terminal manages ephemeral terminal sessions inside the container.
// Terminals are fully in-memory — lost on daemon restart.
package terminal

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/exa-pub/norn/internal/daemon/service/tty"
)

type Terminal struct {
	ID    string
	Name  string
	TTYID string
}

type Service struct {
	ttyMgr tty.Manager

	mu       sync.Mutex
	sessions map[string]*Terminal
}

func NewService(ttyMgr tty.Manager) *Service {
	return &Service{
		ttyMgr:   ttyMgr,
		sessions: make(map[string]*Terminal),
	}
}

func (s *Service) Create(name string, cmd []string) (*Terminal, error) {
	if len(cmd) == 0 {
		cmd = []string{"/bin/bash"}
	}

	termID := uuid.New().String()
	ttySess, err := s.ttyMgr.Create(cmd, func() {
		s.mu.Lock()
		delete(s.sessions, termID)
		s.mu.Unlock()
	})
	if err != nil {
		return nil, fmt.Errorf("create tty: %w", err)
	}

	term := &Terminal{
		ID:    termID,
		Name:  name,
		TTYID: ttySess.ID,
	}

	s.mu.Lock()
	s.sessions[termID] = term
	s.mu.Unlock()

	return term, nil
}

func (s *Service) Delete(terminalID string) error {
	s.mu.Lock()
	t, ok := s.sessions[terminalID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("terminal %q not found", terminalID)
	}
	delete(s.sessions, terminalID)
	s.mu.Unlock()

	if err := s.ttyMgr.Close(t.TTYID); err != nil {
		zap.L().Warn("failed to close tty on terminal delete", zap.String("terminal", terminalID), zap.Error(err))
		return err
	}
	return nil
}

func (s *Service) Rename(terminalID, name string) (*Terminal, error) {
	s.mu.Lock()
	t, ok := s.sessions[terminalID]
	s.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("terminal %q not found", terminalID)
	}
	t.Name = name
	return t, nil
}

func (s *Service) List() []*Terminal {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]*Terminal, 0, len(s.sessions))
	for _, t := range s.sessions {
		result = append(result, t)
	}
	return result
}
