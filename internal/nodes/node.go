package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alib8b8/llm-box/internal/logger"
	"gopkg.in/yaml.v3"
)

// Node defines the interface that all workflow nodes must implement
type Node interface {
	// Name returns the unique identifier of this node
	Name() string

	// Execute runs the node with the given input and parameters
	Execute(ctx context.Context, input string, params map[string]string) (string, error)
}

// NodeMetadata represents the metadata.yaml file for external nodes
type NodeMetadata struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Entry       string `yaml:"entry"`
}

// ExternalNode wraps an external command/script as a Node
type ExternalNode struct {
	metadata NodeMetadata
	nodePath string
}

// NewExternalNode creates a new ExternalNode
func NewExternalNode(metadata NodeMetadata, nodePath string) *ExternalNode {
	return &ExternalNode{
		metadata: metadata,
		nodePath: nodePath,
	}
}

// Name implements the Node interface
func (e *ExternalNode) Name() string {
	return e.metadata.Name
}

// Execute implements the Node interface
func (e *ExternalNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	// Create input payload
	payload := struct {
		Input  string            `json:"input"`
		Params map[string]string `json:"params"`
	}{
		Input:  input,
		Params: params,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to serialize input: %w", err)
	}

	// Build command
	entryPath := filepath.Join(e.nodePath, e.metadata.Entry)
	var cmd *exec.Cmd

	if strings.HasSuffix(entryPath, ".py") {
		cmd = exec.CommandContext(ctx, "python", entryPath)
	} else if strings.HasSuffix(entryPath, ".sh") {
		cmd = exec.CommandContext(ctx, "bash", entryPath)
	} else {
		// Assume binary or executable script
		cmd = exec.CommandContext(ctx, entryPath)
	}

	// Set stdin to payload
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	go func() {
		defer stdin.Close()
		stdin.Write(payloadJSON)
	}()

	// Capture stdout
	stdout, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("node execution failed: %w", err)
	}

	return string(stdout), nil
}

// Registry keeps track of all available nodes
type Registry struct {
	nodes    map[string]Node
	safeMode bool
}

// NewRegistry creates a new registry
func NewRegistry() *Registry {
	return &Registry{
		nodes: make(map[string]Node),
	}
}

// Register adds a node to the registry
func (r *Registry) Register(node Node) {
	r.nodes[node.Name()] = node
}

// Get retrieves a node from the registry by name
func (r *Registry) Get(name string) (Node, bool) {
	node, ok := r.nodes[name]
	return node, ok
}

// List returns all registered node names
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.nodes))
	for name := range r.nodes {
		names = append(names, name)
	}
	return names
}

// SetSafeMode enables or disables safe mode
func (r *Registry) SetSafeMode(enabled bool) {
	r.safeMode = enabled
}

// IsSafeMode returns whether safe mode is enabled
func (r *Registry) IsSafeMode() bool {
	return r.safeMode
}

// LoadExternalNodes scans a directory and loads all external nodes
func (r *Registry) LoadExternalNodes(dir string) error {
	if r.safeMode {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Directory doesn't exist - that's OK
		}
		return fmt.Errorf("failed to read nodes directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue // Skip files, only look at directories
		}

		// Skip directories starting with underscore (e.g. _template)
		if strings.HasPrefix(entry.Name(), "_") {
			continue
		}

		nodeDir := filepath.Join(dir, entry.Name())
		metadataPath := filepath.Join(nodeDir, "metadata.yaml")

		metadataBytes, err := os.ReadFile(metadataPath)
		if err != nil {
			continue // Skip if no metadata.yaml
		}

		var metadata NodeMetadata
		if err := yaml.Unmarshal(metadataBytes, &metadata); err != nil {
			continue // Skip if invalid YAML
		}

		// Check for name collision
		if _, exists := r.nodes[metadata.Name]; exists {
			logger.Warn("external node skipped: name collision", "node", metadata.Name)
			continue
		}

		// Create external node and register
		externalNode := NewExternalNode(metadata, nodeDir)
		r.Register(externalNode)
		logger.Info("loaded external node", "node", metadata.Name, "path", nodeDir)
	}

	return nil
}

// Global registry for backwards compatibility
var globalRegistry = NewRegistry()

// Register adds a node to the global registry
func Register(node Node) {
	globalRegistry.Register(node)
}

// Get retrieves a node from the global registry by name
func Get(name string) (Node, bool) {
	return globalRegistry.Get(name)
}

// List returns all registered node names from the global registry
func List() []string {
	return globalRegistry.List()
}

// SetSafeMode sets safe mode on the global registry
func SetSafeMode(enabled bool) {
	globalRegistry.SetSafeMode(enabled)
}

// IsSafeMode returns whether safe mode is enabled on the global registry
func IsSafeMode() bool {
	return globalRegistry.IsSafeMode()
}

// GetGlobalRegistry returns the global registry
func GetGlobalRegistry() *Registry {
	return globalRegistry
}

// LoadExternalNodes loads external nodes into the global registry
func LoadExternalNodes(dir string) error {
	return globalRegistry.LoadExternalNodes(dir)
}
