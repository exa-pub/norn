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

// FileStore implements Home and InstanceStore
// backed by the filesystem.
type FileStore struct {
	baseDir string
}

func NewFileStore(baseDir string) *FileStore {
	// Resolve to absolute path so that bind-mount source paths passed to
	// devcontainer CLI are unambiguous regardless of --workspace-folder.
	if abs, err := filepath.Abs(baseDir); err == nil {
		baseDir = abs
	}
	s := &FileStore{baseDir: baseDir}
	s.ensureGitignore()
	return s
}

func (s *FileStore) ensureGitignore() {
	p := filepath.Join(s.baseDir, ".gitignore")
	if _, err := os.Stat(p); err == nil {
		return
	}
	_ = os.MkdirAll(s.baseDir, 0o755)
	_ = os.WriteFile(p, []byte("*\n"), 0o644)
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
		filepath.Join(s.InstanceDir(name), "run"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return nil
}
