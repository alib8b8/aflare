package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/alib8b8/llm-box/internal/i18n"
	"github.com/alib8b8/llm-box/internal/logger"
	"gopkg.in/yaml.v3"
)

// ParamSchema describes a single parameter of a node
type ParamSchema struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
}

// NodeSchema describes the input/output schema of a node
type NodeSchema struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Input       string        `json:"input"`
	Output      string        `json:"output"`
	Params      []ParamSchema `json:"params"`
}

// Node defines the interface that all workflow nodes must implement
type Node interface {
	// Name returns the unique identifier of this node
	Name() string

	// Description returns a brief description of this node
	Description() string

	// Schema returns the parameter and I/O schema of this node
	Schema() NodeSchema

	// Execute runs the node with the given input and parameters
	Execute(ctx context.Context, input string, params map[string]string) (string, error)
}

// StreamingNode is an optional interface for nodes that support streaming output
type StreamingNode interface {
	Node
	// ExecuteStream runs the node with streaming output.
	// The onChunk callback is called for each chunk of output.
	// Returns the full output when complete.
	ExecuteStream(ctx context.Context, input string, params map[string]string, onChunk func(chunk string)) (string, error)
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

// Description implements the Node interface
func (e *ExternalNode) Description() string {
	return e.metadata.Description
}

// Schema implements the Node interface
func (e *ExternalNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        e.metadata.Name,
		Description: e.metadata.Description,
		Input:       "string",
		Output:      "string",
		Params:      nil,
	}
}

// Execute implements the Node interface
func (e *ExternalNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	// Create input payload (filter out sensitive keys)
	safeParams := make(map[string]string)
	for k, v := range params {
		// Don't pass API keys to external scripts
		lowerKey := strings.ToLower(k)
		if strings.Contains(lowerKey, "api_key") || strings.Contains(lowerKey, "token") || strings.Contains(lowerKey, "secret") || strings.Contains(lowerKey, "password") {
			continue
		}
		safeParams[k] = v
	}

	payload := struct {
		Input  string            `json:"input"`
		Params map[string]string `json:"params"`
	}{
		Input:  input,
		Params: safeParams,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to serialize input: %w", err)
	}

	// Validate entry path to prevent path traversal
	entryPath := filepath.Join(e.nodePath, e.metadata.Entry)
	absEntryPath, err := filepath.Abs(entryPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve entry path: %w", err)
	}
	// Ensure entry path is within nodePath
	relPath, err := filepath.Rel(e.nodePath, absEntryPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return "", fmt.Errorf("entry path escapes node directory")
	}
	entryPath = absEntryPath

	// Verify file exists and is not a symlink
	info, err := os.Lstat(entryPath)
	if err != nil {
		return "", fmt.Errorf("entry file not found: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("symlink entry files are not allowed")
	}

	var cmd *exec.Cmd

	if strings.HasSuffix(entryPath, ".py") {
		cmd = exec.CommandContext(ctx, "python3", entryPath)
	} else if strings.HasSuffix(entryPath, ".sh") {
		cmd = exec.CommandContext(ctx, "bash", entryPath)
	} else {
		return "", fmt.Errorf("only .py and .sh entry files are allowed")
	}

	// Set stdin via reader instead of pipe to avoid goroutine
	cmd.Stdin = bytes.NewReader(payloadJSON)

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
	mu       sync.RWMutex
}

// NewRegistry creates a new registry
func NewRegistry() *Registry {
	return &Registry{
		nodes: make(map[string]Node),
	}
}

// Register adds a node to the registry
func (r *Registry) Register(node Node) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nodes[node.Name()] = node
}

// Get retrieves a node from the registry by name
func (r *Registry) Get(name string) (Node, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	node, ok := r.nodes[name]
	return node, ok
}

// List returns all registered node names
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.nodes))
	for name := range r.nodes {
		names = append(names, name)
	}
	return names
}

// NodeInfo contains node name and description
type NodeInfo struct {
	Name        string
	Description string
}

// ListNodes returns all registered nodes with their descriptions
func (r *Registry) ListNodes() []NodeInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	infos := make([]NodeInfo, 0, len(r.nodes))
	for _, node := range r.nodes {
		name := node.Name()
		desc := node.Description()
		if i18n.HasKey("node." + name + ".description") {
			desc = i18n.T("node." + name + ".description")
		}
		infos = append(infos, NodeInfo{
			Name:        name,
			Description: desc,
		})
	}
	return infos
}

// SetSafeMode enables or disables safe mode
func (r *Registry) SetSafeMode(enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.safeMode = enabled
}

// IsSafeMode returns whether safe mode is enabled
func (r *Registry) IsSafeMode() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.safeMode
}

// LoadExternalNodes scans a directory and loads all external nodes
func (r *Registry) LoadExternalNodes(dir string) error {
	r.mu.RLock()
	safeMode := r.safeMode
	r.mu.RUnlock()

	if safeMode {
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

		if strings.HasPrefix(entry.Name(), "_") {
			continue
		}

		nodeDir := filepath.Join(dir, entry.Name())

		// Validate path to prevent directory traversal
		safePath, err := safeJoinPath(dir, filepath.Join(entry.Name(), "metadata.yaml"))
		if err != nil {
			continue
		}
		metadataBytes, err := os.ReadFile(safePath)
		if err != nil {
			continue // Skip if no metadata.yaml
		}

		var metadata NodeMetadata
		if err := yaml.Unmarshal(metadataBytes, &metadata); err != nil {
			continue // Skip if invalid YAML
		}

		r.mu.RLock()
		_, exists := r.nodes[metadata.Name]
		r.mu.RUnlock()

		if exists {
			logger.Warn("external node skipped: name collision", "node", metadata.Name)
			continue
		}

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

// RegisterBuiltins registers all built-in nodes to a registry
func RegisterBuiltins(reg *Registry) {
	reg.Register(&ConditionNode{})
	reg.Register(&FetchURLNode{})
	reg.Register(&HTTPRequestNode{})
	reg.Register(&ExecuteNode{})
	reg.Register(&TemplateRenderNode{})
	reg.Register(&FileWriteNode{})
	reg.Register(&FileReadNode{})
	reg.Register(&OpenAINode{})
	reg.Register(&CozeNode{})
	reg.Register(&FastGPTNode{})
	reg.Register(&JSONParseNode{})
	reg.Register(&IMANode{})
	reg.Register(&CombineNode{})
	reg.Register(&TransformNode{})
	reg.Register(&NotifyNode{})
	reg.Register(&OllamaNode{})
	reg.Register(&OpenAICompatibleNode{})
	reg.Register(&CallNode{})
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
