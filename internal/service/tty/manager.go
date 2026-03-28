// Package tty manages ephemeral TTY sessions (PTY processes inside containers).
//
// Each session runs `devcontainer exec` with a host-side PTY (via creack/pty).
// PTY lives independently of WebSocket — closing the browser doesn't kill the process.
//
// A pump goroutine reads PTY output into a ring buffer so that newly attached
// clients can replay recent output instead of seeing a blank screen.
//
// Sessions are ephemeral. On server restart, all sessions are lost.
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

const replayBufSize = 256 * 1024 // 256 KB ring buffer

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
	resize  func(cols, rows uint) error
	detach  func() // removes this subscriber from the session
	Replay  []byte // buffered output to send before live data
}

func (p *PTYStream) Resize(cols, rows uint) error {
	return p.resize(cols, rows)
}

// Detach removes this stream's subscriber. Call when the WebSocket closes.
func (p *PTYStream) Detach() {
	if p.detach != nil {
		p.detach()
	}
}

// subscriber is a pipe that receives live PTY output.
type subscriber struct {
	w *io.PipeWriter
	r *io.PipeReader
}

type session struct {
	id           string
	instanceName string
	process      *devcontainer.ExecProcess
	onClose      func()

	mu          sync.Mutex
	ringBuf     []byte // circular buffer data
	ringPos     int    // write position in ring
	ringFull    bool   // buffer has wrapped at least once
	subscribers map[*subscriber]struct{}
}

func (s *session) addSubscriber(sub *subscriber) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscribers[sub] = struct{}{}
}

func (s *session) removeSubscriber(sub *subscriber) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subscribers, sub)
	_ = sub.w.Close()
}

// snapshot returns a copy of the ring buffer contents in order.
func (s *session) snapshot() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ringFull {
		out := make([]byte, s.ringPos)
		copy(out, s.ringBuf[:s.ringPos])
		return out
	}
	// Buffer has wrapped: [ringPos..end] + [0..ringPos)
	out := make([]byte, len(s.ringBuf))
	n := copy(out, s.ringBuf[s.ringPos:])
	copy(out[n:], s.ringBuf[:s.ringPos])
	return out
}

// writeToRingAndFan writes data to the ring buffer and to all subscribers.
func (s *session) writeToRingAndFan(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Write to ring buffer
	for len(data) > 0 {
		n := copy(s.ringBuf[s.ringPos:], data)
		s.ringPos += n
		if s.ringPos >= len(s.ringBuf) {
			s.ringPos = 0
			s.ringFull = true
		}
		data = data[n:]
	}
}

// fanOut sends data to all subscribers (called without holding mu to avoid blocking PTY reads).
func (s *session) fanOut(data []byte) {
	s.mu.Lock()
	subs := make([]*subscriber, 0, len(s.subscribers))
	for sub := range s.subscribers {
		subs = append(subs, sub)
	}
	s.mu.Unlock()

	for _, sub := range subs {
		// Non-blocking best-effort write. If subscriber is slow, close it.
		_, err := sub.w.Write(data)
		if err != nil {
			s.removeSubscriber(sub)
		}
	}
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
		Cmd:      cmd,
		Cols:     120,
		Rows:     40,
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
		ringBuf:      make([]byte, replayBufSize),
		subscribers:  make(map[*subscriber]struct{}),
	}

	m.mu.Lock()
	m.sessions[sessionID] = sess
	m.mu.Unlock()

	// Pump: read PTY → ring buffer + subscribers
	go m.pump(sessionID, sess)

	return &entity.TTYSession{ID: sessionID}, nil
}

// pump continuously reads from PTY, stores in ring buffer, fans out to subscribers.
func (m *manager) pump(sessionID string, sess *session) {
	buf := make([]byte, 4096)
	for {
		n, err := sess.process.PTY.Read(buf)
		if n > 0 {
			data := buf[:n]
			sess.writeToRingAndFan(data)
			sess.fanOut(data)
		}
		if err != nil {
			break
		}
	}

	// PTY closed — close all subscriber pipes
	sess.mu.Lock()
	for sub := range sess.subscribers {
		_ = sub.w.Close()
	}
	sess.subscribers = make(map[*subscriber]struct{})
	sess.mu.Unlock()

	// Wait for process exit and clean up
	_ = sess.process.Cmd.Wait()

	m.mu.Lock()
	_, stillTracked := m.sessions[sessionID]
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	if stillTracked && sess.onClose != nil {
		sess.onClose()
	}
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

	// Snapshot the ring buffer BEFORE adding subscriber to avoid gaps.
	replay := sess.snapshot()

	// Create a pipe for live data.
	pr, pw := io.Pipe()
	sub := &subscriber{w: pw, r: pr}
	sess.addSubscriber(sub)

	return &PTYStream{
		Reader: pr,
		Writer: sess.process.PTY,
		resize: func(cols, rows uint) error {
			return sess.process.Resize(uint16(cols), uint16(rows))
		},
		detach: func() { sess.removeSubscriber(sub) },
		Replay: replay,
	}, nil
}
