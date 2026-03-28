// Package tty manages ephemeral TTY sessions (PTY processes inside containers).
//
// Each session runs `devcontainer exec` with a host-side PTY (via creack/pty).
// PTY lives independently of WebSocket — closing the browser doesn't kill the process.
//
// Sessions are ephemeral. On server restart, all sessions are lost.
// Orphaned devcontainer exec / docker exec processes will terminate
// when their stdin is closed.
package tty

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/google/uuid"

	"github.com/exa-pub/norn/internal/entity"
	"github.com/exa-pub/norn/internal/pkg/devcontainer"
)

// Manager manages ephemeral TTY sessions.
type Manager interface {
	// Create opens a new PTY inside the container for the given instance.
	// onClose is called when the process exits (in a separate goroutine).
	Create(ctx context.Context, instanceName string, cmd []string, onClose func()) (*entity.TTYSession, error)
	Get(id string) (*entity.TTYSession, bool)
	List(instanceName string) []*entity.TTYSession
	Close(id string) error
	Attach(id string) (*PTYStream, error)
}

// PTYStream is a bidirectional connection to a running PTY.
type PTYStream struct {
	io.Reader
	io.Writer
	resize func(cols, rows uint) error
}

func (p *PTYStream) Resize(cols, rows uint) error {
	return p.resize(cols, rows)
}


type session struct {
	id           string
	instanceName string
	process      *devcontainer.ExecProcess
	onClose      func()
}

type manager struct {
	dc   devcontainer.Client
	opts *devcontainer.GlobalOptions

	mu       sync.Mutex
	sessions map[string]*session
}

func NewManager(dc devcontainer.Client, opts *devcontainer.GlobalOptions) Manager {
	return &manager{
		dc:       dc,
		opts:     opts,
		sessions: make(map[string]*session),
	}
}

func (m *manager) Create(ctx context.Context, instanceName string, cmd []string, onClose func()) (*entity.TTYSession, error) {
	if len(cmd) == 0 {
		cmd = []string{"/bin/bash"}
	}

	proc, err := m.dc.Exec(ctx, devcontainer.ExecOptions{
		Global:   m.opts,
		IDLabels: map[string]string{"norn.name": instanceName},
		Cmd:             cmd,
		Cols:            120,
		Rows:            40,
	})
	if err != nil {
		return nil, fmt.Errorf("exec in %q: %w", instanceName, err)
	}

	sessionID := uuid.New().String()
	sess := &session{
		id:           sessionID,
		instanceName: instanceName,
		process:      proc,
		onClose:      onClose,
	}

	m.mu.Lock()
	m.sessions[sessionID] = sess
	m.mu.Unlock()

	go m.waitForExit(sessionID, sess)

	return &entity.TTYSession{ID: sessionID}, nil
}

func (m *manager) Get(id string) (*entity.TTYSession, bool) {
	m.mu.Lock()
	sess, ok := m.sessions[id]
	m.mu.Unlock()
	if !ok {
		return nil, false
	}
	return &entity.TTYSession{ID: sess.id}, true
}

func (m *manager) List(instanceName string) []*entity.TTYSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*entity.TTYSession
	for _, sess := range m.sessions {
		if sess.instanceName == instanceName {
			result = append(result, &entity.TTYSession{ID: sess.id})
		}
	}
	return result
}

func (m *manager) Close(id string) error {
	m.mu.Lock()
	sess, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("tty session %q: %w", id, entity.ErrNotFound)
	}
	delete(m.sessions, id)
	m.mu.Unlock()

	return sess.process.Close()
}

func (m *manager) Attach(id string) (*PTYStream, error) {
	m.mu.Lock()
	sess, ok := m.sessions[id]
	m.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("tty session %q: %w", id, entity.ErrNotFound)
	}

	return &PTYStream{
		Reader: sess.process.PTY,
		Writer: sess.process.PTY,
		resize: func(cols, rows uint) error {
			return sess.process.Resize(uint16(cols), uint16(rows))
		},
	}, nil
}

func (m *manager) waitForExit(sessionID string, sess *session) {
	_ = sess.process.Cmd.Wait()

	m.mu.Lock()
	_, stillTracked := m.sessions[sessionID]
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	if stillTracked && sess.onClose != nil {
		sess.onClose()
	}
}
