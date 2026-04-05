// Package storage manages daemon-side persistence in the run/ directory.
// Agent session metadata is stored as JSON files in run/agents/.
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AgentMeta is stored in run/agents/{uuid}.json.
type AgentMeta struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Store manages daemon-side file persistence.
type Store struct {
	runDir string // e.g. /mnt/norn/run
}

func New(runDir string) *Store {
	return &Store{runDir: runDir}
}

func (s *Store) agentsDir() string {
	return filepath.Join(s.runDir, "agents")
}

func (s *Store) agentPath(sessionID string) string {
	return filepath.Join(s.agentsDir(), sessionID+".json")
}

func (s *Store) EnsureDirs() error {
	return os.MkdirAll(s.agentsDir(), 0o755)
}

func (s *Store) CreateAgent(meta AgentMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal agent: %w", err)
	}
	return os.WriteFile(s.agentPath(meta.ID), data, 0o644)
}

func (s *Store) ReadAgent(sessionID string) (*AgentMeta, error) {
	data, err := os.ReadFile(s.agentPath(sessionID))
	if err != nil {
		return nil, err
	}
	var meta AgentMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("unmarshal agent: %w", err)
	}
	return &meta, nil
}

func (s *Store) ListAgents() ([]AgentMeta, error) {
	entries, err := os.ReadDir(s.agentsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var result []AgentMeta
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.agentsDir(), e.Name()))
		if err != nil {
			continue
		}
		var meta AgentMeta
		if json.Unmarshal(data, &meta) == nil {
			result = append(result, meta)
		}
	}
	return result, nil
}

func (s *Store) UpdateAgentName(sessionID, name string) error {
	meta, err := s.ReadAgent(sessionID)
	if err != nil {
		return err
	}
	meta.Name = name
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal agent: %w", err)
	}
	return os.WriteFile(s.agentPath(sessionID), data, 0o644)
}

func (s *Store) RemoveAgent(sessionID string) error {
	err := os.Remove(s.agentPath(sessionID))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
