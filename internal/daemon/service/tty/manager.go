// Package tty manages ephemeral TTY sessions inside the container.
//
// Unlike the server-side TTY manager that uses devcontainer exec,
// this runs inside the container and spawns processes directly with a PTY.
//
// A pump goroutine reads PTY output into a ring buffer so that newly attached
// gRPC streams can replay recent output instead of seeing a blank screen.
package tty

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
	"github.com/google/uuid"
)

const replayBufSize = 256 * 1024 // 256 KB ring buffer

// Session represents a running PTY process.
type Session struct {
	ID string
}

// Manager manages ephemeral PTY sessions.
type Manager interface {
	// Create spawns a new process with a PTY. onClose is called when the process exits.
	Create(cmd []string, onClose func()) (*Session, error)
	List() []*Session
	Close(id string) error
	// Attach returns a snapshot of the ring buffer, a reader for live output,
	// and functions for writing input, resizing, and detaching.
	Attach(id string) (*Attachment, error)
}

// Attachment is the result of attaching to a TTY session.
type Attachment struct {
	Replay []byte                         // Ring buffer snapshot
	Reader io.Reader                      // Live output from PTY (subscriber pipe)
	Write  func(data []byte)              // Send input to PTY
	Resize func(cols, rows uint16) error  // Resize PTY window
	Detach func()                         // Unsubscribe and close reader
}

// subscriber receives live PTY output.
type subscriber struct {
	w *io.PipeWriter
	r *io.PipeReader
}

type session struct {
	id      string
	cmd     *exec.Cmd
	ptm     *os.File // PTY master fd
	onClose func()

	mu          sync.Mutex
	ringBuf     []byte
	ringPos     int
	ringFull    bool
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

func (s *session) snapshot() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ringFull {
		out := make([]byte, s.ringPos)
		copy(out, s.ringBuf[:s.ringPos])
		return out
	}
	out := make([]byte, len(s.ringBuf))
	n := copy(out, s.ringBuf[s.ringPos:])
	copy(out[n:], s.ringBuf[:s.ringPos])
	return out
}

func (s *session) writeToRing(data []byte) {
	s.mu.Lock()
	// Ring buffer write
	for len(data) > 0 {
		n := copy(s.ringBuf[s.ringPos:], data)
		s.ringPos += n
		if s.ringPos >= len(s.ringBuf) {
			s.ringPos = 0
			s.ringFull = true
		}
		data = data[n:]
	}
	s.mu.Unlock()
}

func (s *session) fanOut(data []byte) {
	s.mu.Lock()
	subs := make([]*subscriber, 0, len(s.subscribers))
	for sub := range s.subscribers {
		subs = append(subs, sub)
	}
	s.mu.Unlock()

	for _, sub := range subs {
		if _, err := sub.w.Write(data); err != nil {
			s.removeSubscriber(sub)
		}
	}
}

type manager struct {
	mu       sync.Mutex
	sessions map[string]*session
}

func NewManager() Manager {
	return &manager{
		sessions: make(map[string]*session),
	}
}

func (m *manager) Create(cmd []string, onClose func()) (*Session, error) {
	if len(cmd) == 0 {
		cmd = []string{"/bin/bash"}
	}

	c := exec.Command(cmd[0], cmd[1:]...)
	c.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"LANG=en_US.UTF-8",
	)
	ptm, err := pty.StartWithSize(c, &pty.Winsize{Cols: 120, Rows: 40})
	if err != nil {
		return nil, fmt.Errorf("pty start %v: %w", cmd, err)
	}

	sessionID := uuid.New().String()
	sess := &session{
		id:          sessionID,
		cmd:         c,
		ptm:         ptm,
		onClose:     onClose,
		ringBuf:     make([]byte, replayBufSize),
		subscribers: make(map[*subscriber]struct{}),
	}

	m.mu.Lock()
	m.sessions[sessionID] = sess
	m.mu.Unlock()

	go m.pump(sessionID, sess)

	return &Session{ID: sessionID}, nil
}

func (m *manager) pump(sessionID string, sess *session) {
	buf := make([]byte, 4096)
	for {
		n, err := sess.ptm.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			sess.writeToRing(data)
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

	_ = sess.cmd.Wait()

	m.mu.Lock()
	_, stillTracked := m.sessions[sessionID]
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	if stillTracked && sess.onClose != nil {
		sess.onClose()
	}
}

func (m *manager) List() []*Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*Session, 0, len(m.sessions))
	for _, sess := range m.sessions {
		result = append(result, &Session{ID: sess.id})
	}
	return result
}

func (m *manager) Close(id string) error {
	m.mu.Lock()
	sess, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("tty session %q not found", id)
	}
	delete(m.sessions, id)
	m.mu.Unlock()

	_ = sess.ptm.Close()
	return sess.cmd.Process.Kill()
}

func (m *manager) Attach(id string) (*Attachment, error) {
	m.mu.Lock()
	sess, ok := m.sessions[id]
	m.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("tty session %q not found", id)
	}

	replay := sess.snapshot()

	pr, pw := io.Pipe()
	sub := &subscriber{w: pw, r: pr}
	sess.addSubscriber(sub)

	return &Attachment{
		Replay: replay,
		Reader: pr,
		Write: func(data []byte) {
			_, _ = sess.ptm.Write(data)
		},
		Resize: func(cols, rows uint16) error {
			return pty.Setsize(sess.ptm, &pty.Winsize{Cols: cols, Rows: rows})
		},
		Detach: func() {
			sess.removeSubscriber(sub)
		},
	}, nil
}
