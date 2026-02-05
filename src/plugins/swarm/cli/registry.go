package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type SwarmNode struct {
	ID        string    `json:"id"`
	PID       int       `json:"pid"`
	Topic     string    `json:"topic"`
	StartTime time.Time `json:"start_time"`
	Status    string    `json:"status"`
}

type NodeRegistry struct {
	Nodes []SwarmNode `json:"nodes"`
}

func getRegistryPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dialtone", "swarm", "nodes.json")
}

func loadRegistry() (*NodeRegistry, error) {
	path := getRegistryPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &NodeRegistry{Nodes: []SwarmNode{}}, nil
		}
		return nil, err
	}
	var registry NodeRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		return &NodeRegistry{Nodes: []SwarmNode{}}, nil
	}
	return &registry, nil
}

func saveRegistry(registry *NodeRegistry) error {
	path := getRegistryPath()
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
	}
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func addNodeToRegistry(node SwarmNode) error {
	registry, err := loadRegistry()
	if err != nil {
		return err
	}
	registry.Nodes = append(registry.Nodes, node)
	return saveRegistry(registry)
}

func removeNodeFromRegistry(pid int) error {
	registry, err := loadRegistry()
	if err != nil {
		return err
	}
	newNodes := []SwarmNode{}
	for _, n := range registry.Nodes {
		if n.PID != pid {
			newNodes = append(newNodes, n)
		}
	}
	registry.Nodes = newNodes
	return saveRegistry(registry)
}
