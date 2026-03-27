package instance

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// LogEntry is one line from devcontainer up stdout or stderr.
type LogEntry struct {
	Line     string    `json:"line"`
	IsStderr bool      `json:"stderr,omitempty"`
	At       time.Time `json:"at"`
}

// LogWriter captures process output as JSONL to a file.
type LogWriter struct {
	file    *os.File
	done    chan struct{}
	stdoutW *lineWriter
	stderrW *lineWriter
}

func NewLogWriter(path string) (*LogWriter, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}
	lw := &LogWriter{file: f, done: make(chan struct{})}
	lw.stdoutW = &lineWriter{w: lw, isStderr: false}
	lw.stderrW = &lineWriter{w: lw, isStderr: true}
	return lw, nil
}

func (lw *LogWriter) Stdout() io.Writer { return lw.stdoutW }
func (lw *LogWriter) Stderr() io.Writer { return lw.stderrW }
func (lw *LogWriter) Done() <-chan struct{} { return lw.done }

func (lw *LogWriter) Close() {
	lw.stdoutW.flush()
	lw.stderrW.flush()
	_ = lw.file.Close()
	close(lw.done)
}

func (lw *LogWriter) append(entry LogEntry) {
	data, _ := json.Marshal(entry)
	data = append(data, '\n')
	_, _ = lw.file.Write(data)
}

// WatchLogs reads a JSONL log file. If done is non-nil, tails until done closes.
func WatchLogs(ctx context.Context, path string, done <-chan struct{}) <-chan LogEntry {
	ch := make(chan LogEntry, 64)
	go func() {
		defer close(ch)
		f, err := os.Open(path)
		if err != nil {
			return
		}
		defer f.Close()

		sc := bufio.NewScanner(f)
		for {
			for sc.Scan() {
				var e LogEntry
				if json.Unmarshal(sc.Bytes(), &e) != nil {
					continue
				}
				select {
				case ch <- e:
				case <-ctx.Done():
					return
				}
			}
			if done == nil {
				return
			}
			select {
			case <-done:
				sc = bufio.NewScanner(f)
				for sc.Scan() {
					var e LogEntry
					if json.Unmarshal(sc.Bytes(), &e) != nil {
						continue
					}
					select {
					case ch <- e:
					case <-ctx.Done():
						return
					}
				}
				return
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
				sc = bufio.NewScanner(f)
			}
		}
	}()
	return ch
}

type lineWriter struct {
	w        *LogWriter
	isStderr bool
	pending  []byte
}

func (lw *lineWriter) Write(p []byte) (int, error) {
	lw.pending = append(lw.pending, p...)
	for {
		i := bytes.IndexByte(lw.pending, '\n')
		if i < 0 {
			break
		}
		lw.w.append(LogEntry{Line: string(lw.pending[:i]), IsStderr: lw.isStderr, At: time.Now()})
		lw.pending = lw.pending[i+1:]
	}
	return len(p), nil
}

func (lw *lineWriter) flush() {
	if len(lw.pending) > 0 {
		lw.w.append(LogEntry{Line: string(lw.pending), IsStderr: lw.isStderr, At: time.Now()})
		lw.pending = nil
	}
}
