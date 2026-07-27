// Copyright (c) 2026 llm-box Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Package core contains the shared infrastructure for workflow nodes:
// the Node interface, Registry, security helpers, LLM base client, and
// common parameter helpers. Sub-packages under internal/nodes/ should
// import this package instead of the parent nodes package to avoid
// circular imports.
package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

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
		if IsSensitiveKey(k) {
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

	// Capture stdout and stderr separately
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("node execution failed: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// NodeExecStats tracks execution statistics for a single node
type NodeExecStats struct {
	Calls       int64
	Errors      int64
	TotalMs     int64
	InputBytes  int64
	OutputBytes int64
}

// Registry keeps track of all available nodes
type Registry struct {
	nodes    map[string]Node
	safeMode bool
	mu       sync.RWMutex
	stats    map[string]*NodeExecStats
	statsMu  sync.RWMutex
}

// NewRegistry creates a new registry
func NewRegistry() *Registry {
	return &Registry{
		nodes: make(map[string]Node),
		stats: make(map[string]*NodeExecStats),
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
	sort.Strings(names)
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
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})
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

// ExecuteWithStats runs a node and records execution metrics (calls, errors, latency, I/O size)
func (r *Registry) ExecuteWithStats(name string, ctx context.Context, input string, params map[string]string) (string, error) {
	node, ok := r.Get(name)
	if !ok {
		return "", fmt.Errorf("node %q not found", name)
	}

	start := time.Now()
	output, err := node.Execute(ctx, input, params)
	elapsed := time.Since(start).Milliseconds()

	r.statsMu.Lock()
	defer r.statsMu.Unlock()

	if r.stats[name] == nil {
		r.stats[name] = &NodeExecStats{}
	}
	s := r.stats[name]
	s.Calls++
	s.TotalMs += elapsed
	s.InputBytes += int64(len(input))
	s.OutputBytes += int64(len(output))
	if err != nil {
		s.Errors++
	}

	return output, err
}

// GetStats returns execution statistics for a node
func (r *Registry) GetStats(name string) *NodeExecStats {
	r.statsMu.RLock()
	defer r.statsMu.RUnlock()
	if s, ok := r.stats[name]; ok {
		cp := *s
		return &cp
	}
	return nil
}

// GetAllStats returns execution statistics for all nodes
func (r *Registry) GetAllStats() map[string]NodeExecStats {
	r.statsMu.RLock()
	defer r.statsMu.RUnlock()
	result := make(map[string]NodeExecStats, len(r.stats))
	for name, s := range r.stats {
		result[name] = *s
	}
	return result
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
		safePath, err := SafeJoinPath(dir, filepath.Join(entry.Name(), "metadata.yaml"))
		if err != nil {
			continue
		}
		// Open the file and fstat the fd to avoid TOCTOU race (symlink swap
		// between path validation and read).
		f, err := os.Open(safePath)
		if err != nil {
			continue // Skip if no metadata.yaml
		}
		info, err := f.Stat()
		if err != nil {
			_ = f.Close() // best-effort close
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			_ = f.Close() // best-effort close
			continue
		}
		metadataBytes, err := io.ReadAll(f)
		_ = f.Close() // best-effort close
		if err != nil {
			continue
		}

		var metadata NodeMetadata
		if err := yaml.Unmarshal(metadataBytes, &metadata); err != nil {
			continue // Skip if invalid YAML
		}

		r.mu.RLock()
		exists := r.nodes[metadata.Name] != nil
		safeModeNow := r.safeMode
		r.mu.RUnlock()

		if exists {
			logger.Warn("external node skipped: name collision", "node", metadata.Name)
			continue
		}
		if safeModeNow {
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

// NodeCategory labels a node's functional category (llm, agent, io, etc.).
// It is used by Registry.Search and Registry.NodesByCategory.
type NodeCategory string

const (
	// CategoryLLM groups LLM provider nodes.
	CategoryLLM NodeCategory = "llm"
	// CategoryAgent groups agent / ReAct-style nodes.
	CategoryAgent NodeCategory = "agent"
	// CategoryIO groups input/output nodes (file_read, fetch_url, etc.).
	CategoryIO NodeCategory = "io"
	// CategoryTransform groups data-transformation nodes.
	CategoryTransform NodeCategory = "transform"
	// CategoryFlow groups control-flow nodes (if/switch/loop).
	CategoryFlow NodeCategory = "flow"
	// CategoryData groups data/storage nodes (rag, knowledge_graph, etc.).
	CategoryData NodeCategory = "data"
	// CategorySecurity groups security-related nodes (hash, sign, etc.).
	CategorySecurity NodeCategory = "security"
	// CategoryUtility groups miscellaneous utility nodes.
	CategoryUtility NodeCategory = "utility"
)

// Search returns the registered nodes whose Name or Description contain
// query (case-insensitive), sorted by Name.
func (r *Registry) Search(query string) []NodeInfo {
	queryLower := strings.ToLower(query)
	var matched []NodeInfo
	for _, info := range r.ListNodes() {
		nameLower := strings.ToLower(info.Name)
		descLower := strings.ToLower(info.Description)
		if strings.Contains(nameLower, queryLower) || strings.Contains(descLower, queryLower) {
			matched = append(matched, info)
		}
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Name < matched[j].Name
	})
	return matched
}

// NodesByCategory returns the registered nodes that belong to the given
// category, based on a hardcoded name->category mapping maintained here
// so that the marketplace node and the CLI can share it.
func (r *Registry) NodesByCategory(category NodeCategory) []NodeInfo {
	categoryMap := map[NodeCategory][]string{
		CategoryLLM: {
			"ollama", "openai", "deepseek", "glm", "kimi", "qwen", "mistral", "yi",
			"anthropic", "gemini", "cohere", "together", "groq",
		},
		CategoryAgent: {
			"agent", "supervisor", "planner", "researcher", "critic",
			"evaluator", "reflector", "code_review",
		},
		CategoryIO: {
			"file_read", "file_write", "file_append", "file_list",
			"fetch_url", "http_request", "stdin", "stdout", "output",
		},
		CategoryTransform: {
			"json_parse", "transform", "combine", "template_render",
			"markdown_render", "base64_encode", "base64_decode",
		},
		CategoryFlow: {
			"if", "switch", "loop", "wait", "parallel", "map",
		},
		CategoryData: {
			"rag", "knowledge_graph", "smart_router", "code_interpreter",
			"execute", "multimodal", "node_marketplace",
		},
		CategorySecurity: {
			"hash", "encrypt", "decrypt", "sign", "verify",
		},
		CategoryUtility: {
			"echo", "wait", "log", "env", "variable",
		},
	}

	nodeNames, ok := categoryMap[category]
	if !ok {
		return nil
	}

	var result []NodeInfo
	for _, name := range nodeNames {
		if node, exists := r.Get(name); exists {
			desc := node.Description()
			result = append(result, NodeInfo{Name: name, Description: desc})
		}
	}
	return result
}
