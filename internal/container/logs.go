package container

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogEntry is one line from devcontainer up stdout or stderr.
type LogEntry struct {
	Line     string    `json:"line"`
	IsStderr bool      `json:"stderr,omitempty"`
	At       time.Time `json:"at"`
}

// LogBus captures startup logs to files and fans them out
// to any number of live subscribers by tailing the log file on disk.
type LogBus struct {
	mu     sync.Mutex
	closed bool
	signal chan struct{} // closed+replaced on each write to wake subscribers

	logPath    string   // combined JSONL file (source of truth)
	logFile    *os.File // write handle for JSONL
	stdoutFile *os.File
	stderrFile *os.File
	stdoutW    *logWriter
	stderrW    *logWriter
}

// newLogBus opens log files and returns a ready LogBus.
func newLogBus(logDir, prefix string) (*LogBus, error) {
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir logs: %w", err)
	}

	logFile, err := os.OpenFile(
		filepath.Join(logDir, prefix+".log.jsonl"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644,
	)
	if err != nil {
		return nil, fmt.Errorf("open jsonl log: %w", err)
	}
	stdoutFile, err := os.OpenFile(
		filepath.Join(logDir, prefix+".stdout.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644,
	)
	if err != nil {
		logFile.Close()
		return nil, fmt.Errorf("open stdout log: %w", err)
	}
	stderrFile, err := os.OpenFile(
		filepath.Join(logDir, prefix+".stderr.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644,
	)
	if err != nil {
		logFile.Close()
		stdoutFile.Close()
		return nil, fmt.Errorf("open stderr log: %w", err)
	}

	b := &LogBus{
		signal:     make(chan struct{}),
		logPath:    logFile.Name(),
		logFile:    logFile,
		stdoutFile: stdoutFile,
		stderrFile: stderrFile,
	}
	b.stdoutW = &logWriter{file: stdoutFile, bus: b, isStderr: false}
	b.stderrW = &logWriter{file: stderrFile, bus: b, isStderr: true}
	return b, nil
}

// Stdout returns a writer for the stdout stream.
func (b *LogBus) Stdout() io.Writer { return b.stdoutW }

// Stderr returns a writer for the stderr stream.
func (b *LogBus) Stderr() io.Writer { return b.stderrW }

// Subscribe tails the JSONL log file from the beginning.
// Returns a channel that delivers all entries (past and live).
// The channel is closed when the bus is closed or ctx is cancelled.
func (b *LogBus) Subscribe(ctx context.Context) <-chan LogEntry {
	ch := make(chan LogEntry, 64)

	go func() {
		defer close(ch)

		f, err := os.Open(b.logPath)
		if err != nil {
			return
		}
		defer f.Close()

		var pending []byte
		buf := make([]byte, 4096)
		for {
			// Read all available data.
			for {
				n, _ := f.Read(buf)
				if n == 0 {
					break
				}
				pending = append(pending, buf[:n]...)

				// Emit complete JSONL lines.
				for {
					idx := bytes.IndexByte(pending, '\n')
					if idx < 0 {
						break
					}
					line := pending[:idx]
					pending = pending[idx+1:]

					var entry LogEntry
					if json.Unmarshal(line, &entry) != nil {
						continue
					}
					select {
					case ch <- entry:
					case <-ctx.Done():
						return
					}
				}
			}

			// Wait for new data or shutdown.
			b.mu.Lock()
			if b.closed {
				b.mu.Unlock()
				return
			}
			sig := b.signal
			b.mu.Unlock()

			select {
			case <-sig:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch
}

// Close flushes partial lines, closes files, and wakes all subscribers.
func (b *LogBus) Close() {
	b.stdoutW.flush()
	b.stderrW.flush()

	b.mu.Lock()
	defer b.mu.Unlock()

	b.closed = true
	close(b.signal)

	_ = b.logFile.Close()
	_ = b.stdoutFile.Close()
	_ = b.stderrFile.Close()
}

// appendEntry writes a LogEntry as JSONL and notifies subscribers.
func (b *LogBus) appendEntry(entry LogEntry) {
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	data = append(data, '\n')

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}

	_, _ = b.logFile.Write(data)

	close(b.signal)
	b.signal = make(chan struct{})
}

// logWriter writes raw bytes to a file and splits them into lines,
// appending each complete line as a LogEntry to the JSONL log.
type logWriter struct {
	file     *os.File
	bus      *LogBus
	isStderr bool
	pending  []byte
}

func (w *logWriter) Write(p []byte) (int, error) {
	n, err := w.file.Write(p)
	if n > 0 {
		w.pending = append(w.pending, p[:n]...)
		w.emitLines()
	}
	return n, err
}

func (w *logWriter) emitLines() {
	for {
		i := bytes.IndexByte(w.pending, '\n')
		if i < 0 {
			break
		}
		line := string(w.pending[:i])
		w.pending = w.pending[i+1:]
		w.bus.appendEntry(LogEntry{Line: line, IsStderr: w.isStderr, At: time.Now()})
	}
}

func (w *logWriter) flush() {
	if len(w.pending) > 0 {
		w.bus.appendEntry(LogEntry{
			Line:     string(w.pending),
			IsStderr: w.isStderr,
			At:       time.Now(),
		})
		w.pending = nil
	}
}
