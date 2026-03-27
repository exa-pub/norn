package tty

import (
	"context"
	"fmt"
	"io"
	"sync"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/google/uuid"

	"github.com/exa-pub/norn/internal/entity"
	"github.com/exa-pub/norn/pkg/dockerutils"
)

// Manager manages ephemeral TTY sessions (PTY processes inside containers).
// PTY lives independently of WebSocket.
type Manager interface {
	Create(ctx context.Context, instanceName string, cmd []string) (*entity.TTYSession, error)
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
	name         string
	instanceName string
	hijack       dockertypes.HijackedResponse
	execID       string
}

type manager struct {
	docker *client.Client

	mu       sync.Mutex
	sessions map[string]*session
}

func NewManager(dk *client.Client) Manager {
	return &manager{
		docker:   dk,
		sessions: make(map[string]*session),
	}
}

func (m *manager) Create(ctx context.Context, instanceName string, cmd []string) (*entity.TTYSession, error) {
	dc, err := dockerutils.FindByLabel(ctx, m.docker, "norn.name", instanceName)
	if err != nil {
		return nil, fmt.Errorf("docker: %w", err)
	}
	if dc == nil || dc.State != "running" {
		return nil, fmt.Errorf("instance %q has no running container: %w", instanceName, entity.ErrFailedPrecondition)
	}

	if len(cmd) == 0 {
		cmd = []string{"/bin/bash"}
	}

	execResp, err := m.docker.ContainerExecCreate(ctx, dc.ID, container.ExecOptions{
		Cmd:          cmd,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("exec create: %w", err)
	}

	hijack, err := m.docker.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{Tty: true})
	if err != nil {
		return nil, fmt.Errorf("exec attach: %w", err)
	}

	sessionID := uuid.New().String()
	sess := &session{
		id:           sessionID,
		instanceName: instanceName,
		hijack:       hijack,
		execID:       execResp.ID,
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
	return &entity.TTYSession{ID: sess.id, Name: sess.name}, true
}

func (m *manager) List(instanceName string) []*entity.TTYSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*entity.TTYSession
	for _, sess := range m.sessions {
		if sess.instanceName == instanceName {
			result = append(result, &entity.TTYSession{ID: sess.id, Name: sess.name})
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

	sess.hijack.Close()
	return nil
}

func (m *manager) Attach(id string) (*PTYStream, error) {
	m.mu.Lock()
	sess, ok := m.sessions[id]
	m.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("tty session %q: %w", id, entity.ErrNotFound)
	}

	execID := sess.execID
	docker := m.docker
	return &PTYStream{
		Reader: sess.hijack.Reader,
		Writer: sess.hijack.Conn,
		resize: func(cols, rows uint) error {
			return docker.ContainerExecResize(context.Background(), execID, container.ResizeOptions{
				Width:  cols,
				Height: rows,
			})
		},
	}, nil
}

func (m *manager) waitForExit(sessionID string, sess *session) {
	// Read until EOF — process exited and hijacked connection closed.
	buf := make([]byte, 256)
	for {
		if _, err := sess.hijack.Reader.Read(buf); err != nil {
			break
		}
	}

	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()
}
