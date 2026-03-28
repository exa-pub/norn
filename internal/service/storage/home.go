package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// Home manages the NornHome directory layout.
type Home interface {
	BaseDir() string
	InstanceDir(name string) string
	EnsureInstanceDirs(name string) error
}

// FileStore implements Home, InstanceStore, and AgentStore
// backed by the filesystem.
type FileStore struct {
	baseDir string
}

func NewFileStore(baseDir string) *FileStore {
	return &FileStore{baseDir: baseDir}
}

func (s *FileStore) BaseDir() string {
	return s.baseDir
}

func (s *FileStore) InstanceDir(name string) string {
	return filepath.Join(s.baseDir, "instances", name)
}

func (s *FileStore) EnsureInstanceDirs(name string) error {
	dirs := []string{
		filepath.Join(s.baseDir, "shared", "dotfiles"),
		filepath.Join(s.InstanceDir(name), "mnt"),
		filepath.Join(s.InstanceDir(name), "logs"),
		filepath.Join(s.InstanceDir(name), "agents"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return nil
}
