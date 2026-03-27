package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AgentMeta is stored in agents/{uuid}.json.
type AgentMeta struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// AgentStore manages Agent Session persistence.
type AgentStore interface {
	CreateAgent(instanceName string, meta AgentMeta) error
	ReadAgent(instanceName, sessionID string) (*AgentMeta, error)
	ListAgents(instanceName string) ([]AgentMeta, error)
	UpdateAgentName(instanceName, sessionID, name string) error
	RemoveAgent(instanceName, sessionID string) error
}

// --- AgentStore implementation on FileStore ---

func (s *FileStore) CreateAgent(instanceName string, meta AgentMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal agent: %w", err)
	}
	return os.WriteFile(s.agentPath(instanceName, meta.ID), data, 0o644)
}

func (s *FileStore) ReadAgent(instanceName, sessionID string) (*AgentMeta, error) {
	data, err := os.ReadFile(s.agentPath(instanceName, sessionID))
	if err != nil {
		return nil, err
	}
	var meta AgentMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("unmarshal agent: %w", err)
	}
	return &meta, nil
}

func (s *FileStore) ListAgents(instanceName string) ([]AgentMeta, error) {
	dir := filepath.Join(s.InstanceDir(instanceName), "agents")
	entries, err := os.ReadDir(dir)
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
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
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

func (s *FileStore) UpdateAgentName(instanceName, sessionID, name string) error {
	meta, err := s.ReadAgent(instanceName, sessionID)
	if err != nil {
		return err
	}
	meta.Name = name
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal agent: %w", err)
	}
	return os.WriteFile(s.agentPath(instanceName, sessionID), data, 0o644)
}

func (s *FileStore) RemoveAgent(instanceName, sessionID string) error {
	err := os.Remove(s.agentPath(instanceName, sessionID))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *FileStore) agentPath(instanceName, sessionID string) string {
	return filepath.Join(s.InstanceDir(instanceName), "agents", sessionID+".json")
}
