// Package agent manages AI agent sessions inside the container.
// The daemon owns both metadata (persisted in run/agents/) and
// runtime state (PTY processes).
package agent

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/exa-pub/norn/internal/daemon/service/storage"
	"github.com/exa-pub/norn/internal/daemon/service/tty"
)

// AgentSession is the daemon-side view of an agent session.
type AgentSession struct {
	ID      string
	Name    string
	Running bool
	TTYID   string
}

type Service struct {
	store  *storage.Store
	ttyMgr tty.Manager

	mu      sync.Mutex
	running map[string]string // sessionID → ttyID
}

func NewService(store *storage.Store, ttyMgr tty.Manager) *Service {
	return &Service{
		store:   store,
		ttyMgr:  ttyMgr,
		running: make(map[string]string),
	}
}

func (s *Service) Create(name, prompt string) (*AgentSession, error) {
	sessionID := uuid.New().String()
	meta := storage.AgentMeta{ID: sessionID, Name: name}
	if err := s.store.CreateAgent(meta); err != nil {
		return nil, fmt.Errorf("create agent: %w", err)
	}

	// Build claude command
	cmd := []string{
		"claude",
		"--dangerously-skip-permissions",
		"--session-id", sessionID,
	}
	if prompt != "" {
		cmd = append(cmd, "-p", prompt)
	}

	var ttyID string
	ttySess, err := s.ttyMgr.Create(cmd, func() {
		s.mu.Lock()
		if s.running[sessionID] == ttyID {
			delete(s.running, sessionID)
		}
		s.mu.Unlock()
	})
	if err != nil {
		// Cleanup metadata on PTY failure
		_ = s.store.RemoveAgent(sessionID)
		return nil, fmt.Errorf("create tty for agent: %w", err)
	}
	ttyID = ttySess.ID

	s.mu.Lock()
	s.running[sessionID] = ttyID
	s.mu.Unlock()

	return &AgentSession{
		ID: sessionID, Name: name,
		Running: true, TTYID: ttyID,
	}, nil
}

func (s *Service) Start(sessionID, prompt string) (*AgentSession, error) {
	meta, err := s.store.ReadAgent(sessionID)
	if err != nil {
		return nil, fmt.Errorf("agent session %q not found", sessionID)
	}

	s.mu.Lock()
	if _, already := s.running[sessionID]; already {
		s.mu.Unlock()
		return nil, fmt.Errorf("agent session %q already running", sessionID)
	}
	s.mu.Unlock()

	cmd := []string{
		"claude",
		"--dangerously-skip-permissions",
		"--resume", sessionID,
	}
	if prompt != "" {
		cmd = append(cmd, "-p", prompt)
	}

	var ttyID string
	ttySess, err := s.ttyMgr.Create(cmd, func() {
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

	return &AgentSession{
		ID: meta.ID, Name: meta.Name,
		Running: true, TTYID: ttyID,
	}, nil
}

func (s *Service) Stop(sessionID string) (*AgentSession, error) {
	meta, err := s.store.ReadAgent(sessionID)
	if err != nil {
		return nil, fmt.Errorf("agent session %q not found", sessionID)
	}

	s.mu.Lock()
	ttyID, running := s.running[sessionID]
	if running {
		delete(s.running, sessionID)
	}
	s.mu.Unlock()

	if running {
		if err := s.ttyMgr.Close(ttyID); err != nil {
			zap.L().Warn("failed to close tty on agent stop", zap.String("session", sessionID), zap.Error(err))
		}
	}

	return &AgentSession{ID: meta.ID, Name: meta.Name, Running: false}, nil
}

func (s *Service) Delete(sessionID string) error {
	// Stop if running
	s.mu.Lock()
	ttyID, running := s.running[sessionID]
	if running {
		delete(s.running, sessionID)
	}
	s.mu.Unlock()

	if running {
		if err := s.ttyMgr.Close(ttyID); err != nil {
			zap.L().Warn("failed to close tty on agent delete", zap.String("session", sessionID), zap.Error(err))
		}
	}

	return s.store.RemoveAgent(sessionID)
}

func (s *Service) Rename(sessionID, name string) (*AgentSession, error) {
	if err := s.store.UpdateAgentName(sessionID, name); err != nil {
		return nil, fmt.Errorf("agent session %q not found", sessionID)
	}
	return s.toSession(sessionID, name), nil
}

func (s *Service) Get(sessionID string) (*AgentSession, error) {
	meta, err := s.store.ReadAgent(sessionID)
	if err != nil {
		return nil, fmt.Errorf("agent session %q not found", sessionID)
	}
	return s.toSession(meta.ID, meta.Name), nil
}

func (s *Service) List() ([]*AgentSession, error) {
	metas, err := s.store.ListAgents()
	if err != nil {
		return nil, err
	}
	result := make([]*AgentSession, 0, len(metas))
	for _, m := range metas {
		result = append(result, s.toSession(m.ID, m.Name))
	}
	return result, nil
}

func (s *Service) toSession(id, name string) *AgentSession {
	s.mu.Lock()
	ttyID, running := s.running[id]
	s.mu.Unlock()
	return &AgentSession{
		ID: id, Name: name,
		Running: running, TTYID: ttyID,
	}
}
