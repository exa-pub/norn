package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// InstanceMeta is stored in identity.json. Write-once.
type InstanceMeta struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// InstanceStore manages Instance persistence: identity, errors, log paths.
type InstanceStore interface {
	Create(name string, meta InstanceMeta) error
	Read(name string) (*InstanceMeta, error)
	Exists(name string) bool
	List() ([]string, error)
	Remove(name string) error

	NewLogPath(name string, ts time.Time, source string) (string, error)
	LastLogPath(name string) string
	ListLogFiles(name string) ([]LogFileInfo, error)
	LogPathByID(name, logID string) string
	WriteError(name string, msg string) error
	ReadError(name string) string
	ClearError(name string) error
}

// --- InstanceStore implementation on FileStore ---

func (s *FileStore) Create(name string, meta InstanceMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal identity: %w", err)
	}
	return os.WriteFile(s.identityPath(name), data, 0o644)
}

func (s *FileStore) Read(name string) (*InstanceMeta, error) {
	data, err := os.ReadFile(s.identityPath(name))
	if err != nil {
		return nil, err
	}
	var meta InstanceMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("unmarshal identity: %w", err)
	}
	return &meta, nil
}

func (s *FileStore) Exists(name string) bool {
	_, err := os.Stat(s.identityPath(name))
	return err == nil
}

func (s *FileStore) List() ([]string, error) {
	dir := filepath.Join(s.baseDir, "instances")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

func (s *FileStore) Remove(name string) error {
	return os.RemoveAll(s.InstanceDir(name))
}

func (s *FileStore) NewLogPath(name string, ts time.Time, source string) (string, error) {
	logDir := filepath.Join(s.InstanceDir(name), "logs", ts.Format("2006-01-02"))
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir logs: %w", err)
	}
	return filepath.Join(logDir, ts.Format("15-04-05")+"."+source+".jsonl"), nil
}

func (s *FileStore) LastLogPath(name string) string {
	logsDir := filepath.Join(s.InstanceDir(name), "logs")
	dayDirs, err := os.ReadDir(logsDir)
	if err != nil || len(dayDirs) == 0 {
		return ""
	}
	sort.Slice(dayDirs, func(i, j int) bool {
		return dayDirs[i].Name() > dayDirs[j].Name()
	})
	for _, dayDir := range dayDirs {
		if !dayDir.IsDir() {
			continue
		}
		dayPath := filepath.Join(logsDir, dayDir.Name())
		files, err := os.ReadDir(dayPath)
		if err != nil || len(files) == 0 {
			continue
		}
		sort.Slice(files, func(i, j int) bool {
			return files[i].Name() > files[j].Name()
		})
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".jsonl") {
				return filepath.Join(dayPath, f.Name())
			}
		}
	}
	return ""
}

// LogFileInfo describes a single log file.
type LogFileInfo struct {
	ID     string // e.g. "2026-04-05/14-30-00.daemon"
	Source string // e.g. "daemon", "up", or "" for legacy
	Date   string // e.g. "2026-04-05"
	Time   string // e.g. "14:30:00"
	Size   int64
}

// ListLogFiles returns all log files for an instance, sorted newest first.
func (s *FileStore) ListLogFiles(name string) ([]LogFileInfo, error) {
	logsDir := filepath.Join(s.InstanceDir(name), "logs")
	dayDirs, err := os.ReadDir(logsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	sort.Slice(dayDirs, func(i, j int) bool {
		return dayDirs[i].Name() > dayDirs[j].Name()
	})

	var result []LogFileInfo
	for _, dayDir := range dayDirs {
		if !dayDir.IsDir() {
			continue
		}
		date := dayDir.Name()
		dayPath := filepath.Join(logsDir, date)
		files, err := os.ReadDir(dayPath)
		if err != nil {
			continue
		}
		sort.Slice(files, func(i, j int) bool {
			return files[i].Name() > files[j].Name()
		})
		for _, f := range files {
			if !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}
			base := strings.TrimSuffix(f.Name(), ".jsonl")
			// Parse: "HH-mm-ss.source" or legacy "HH-mm-ss"
			var timePart, source string
			if dotIdx := strings.IndexByte(base, '.'); dotIdx >= 0 {
				timePart = base[:dotIdx]
				source = base[dotIdx+1:]
			} else {
				timePart = base
				source = "up" // legacy files without source
			}
			timeFormatted := strings.ReplaceAll(timePart, "-", ":")
			var size int64
			if info, err := f.Info(); err == nil {
				size = info.Size()
			}
			result = append(result, LogFileInfo{
				ID:     date + "/" + base,
				Source: source,
				Date:   date,
				Time:   timeFormatted,
				Size:   size,
			})
		}
	}
	return result, nil
}

// LogPathByID resolves a log ID (e.g. "2026-04-05/14-30-00.daemon") to its file path.
func (s *FileStore) LogPathByID(name, logID string) string {
	return filepath.Join(s.InstanceDir(name), "logs", logID+".jsonl")
}

func (s *FileStore) WriteError(name string, msg string) error {
	return os.WriteFile(s.errorPath(name), []byte(msg), 0o644)
}

func (s *FileStore) ReadError(name string) string {
	data, err := os.ReadFile(s.errorPath(name))
	if err != nil {
		return ""
	}
	return string(data)
}

func (s *FileStore) ClearError(name string) error {
	err := os.Remove(s.errorPath(name))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *FileStore) identityPath(name string) string {
	return filepath.Join(s.InstanceDir(name), "identity.json")
}

func (s *FileStore) errorPath(name string) string {
	return filepath.Join(s.InstanceDir(name), "last_error")
}
