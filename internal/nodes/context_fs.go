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

package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/alib8b8/llm-box/internal/memory"
)

// validContextFSOperations is the set of supported filesystem operations.
var validContextFSOperations = map[string]bool{
	"ls":     true,
	"cat":    true,
	"write":  true,
	"rm":     true,
	"search": true,
}

// globalFSStore is the singleton FSStore used by ContextFSNode. A single
// instance is shared across all node invocations so that the in-memory
// knowledge graph cache stays warm and writes are immediately visible to
// subsequent reads.
var (
	globalFSStore     *memory.FSStore
	globalFSStoreOnce sync.Once
)

// getGlobalFSStore lazily initializes and returns the shared FSStore.
func getGlobalFSStore() *memory.FSStore {
	globalFSStoreOnce.Do(func() {
		globalFSStore = memory.NewFSStore("")
	})
	return globalFSStore
}

// ContextFSNode exposes the unified context filesystem as a workflow node.
// Inspired by ByteDance OpenViking, it lets agents interact with memory,
// user profile, knowledge graph, and skills through a single filesystem
// paradigm (ls/cat/write/rm/search) instead of three different APIs.
type ContextFSNode struct{}

func init() {
	Register(&ContextFSNode{})
}

// Name returns the node name.
func (n *ContextFSNode) Name() string { return "context_fs" }

// Description returns a human-readable description of the node.
func (n *ContextFSNode) Description() string {
	return "Unified context filesystem (OpenViking-inspired): ls/cat/write/rm/search over memory, profile, knowledge graph, and skills via a single virtual path namespace."
}

// Schema returns the node's input/output/params schema.
func (n *ContextFSNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - content for write op, or query for search op (used when content/query params are absent)",
		Output:      "string - file content (cat), JSON entries (ls/search), or status message (write/rm)",
		Params: []ParamSchema{
			{Name: "operation", Type: "string", Description: "ls|cat|write|rm|search (default: ls)", Required: false, Default: "ls"},
			{Name: "path", Type: "string", Description: "Virtual path, e.g. /mem/short/note, /profile/coding_style/lang, /kg/entities/foo, /skills/bar, or / for root listing", Required: false},
			{Name: "content", Type: "string", Description: "Content to write (defaults to input)", Required: false},
			{Name: "query", Type: "string", Description: "Search query (defaults to input)", Required: false},
			{Name: "scope", Type: "string", Description: "Search scope: /mem, /profile, /kg, /skills, or empty for all", Required: false},
			{Name: "top_k", Type: "int", Description: "Max results for search (default: 10)", Required: false, Default: "10"},
		},
	}
}

// Execute runs the requested filesystem operation against the shared FSStore.
func (n *ContextFSNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	operation := getParam(params, "operation", "ls")
	if !validContextFSOperations[operation] {
		return "", fmt.Errorf("invalid operation: %s (supported: ls, cat, write, rm, search)", operation)
	}

	store := getGlobalFSStore()

	switch operation {
	case "ls":
		return n.executeList(store, params)
	case "cat":
		return n.executeCat(store, params)
	case "write":
		return n.executeWrite(store, input, params)
	case "rm":
		return n.executeRm(store, params)
	case "search":
		return n.executeSearch(store, input, params)
	default:
		return "", fmt.Errorf("unsupported operation: %s", operation)
	}
}

// executeList handles the ls operation, returning JSON-serialized FSEntry slice.
func (n *ContextFSNode) executeList(store *memory.FSStore, params map[string]string) (string, error) {
	path := getParam(params, "path", "/")
	entries, err := store.List(path)
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal entries: %w", err)
	}
	return string(data), nil
}

// executeCat handles the cat operation, returning raw file content.
func (n *ContextFSNode) executeCat(store *memory.FSStore, params map[string]string) (string, error) {
	path := getParam(params, "path", "")
	if path == "" {
		return "", fmt.Errorf("path is required for cat operation")
	}
	return store.Read(path)
}

// executeWrite handles the write operation, storing content at the given path.
func (n *ContextFSNode) executeWrite(store *memory.FSStore, input string, params map[string]string) (string, error) {
	path := getParam(params, "path", "")
	if path == "" {
		return "", fmt.Errorf("path is required for write operation")
	}
	content := getParam(params, "content", "")
	if content == "" {
		content = input
	}
	if content == "" {
		return "", fmt.Errorf("content is required for write operation")
	}
	if err := store.Write(path, content); err != nil {
		return "", err
	}
	return fmt.Sprintf("written %d bytes to %s", len(content), path), nil
}

// executeRm handles the rm operation, deleting the given path.
func (n *ContextFSNode) executeRm(store *memory.FSStore, params map[string]string) (string, error) {
	path := getParam(params, "path", "")
	if path == "" {
		return "", fmt.Errorf("path is required for rm operation")
	}
	if err := store.Delete(path); err != nil {
		return "", err
	}
	return fmt.Sprintf("deleted %s", path), nil
}

// executeSearch handles the search operation, returning JSON-serialized matches.
func (n *ContextFSNode) executeSearch(store *memory.FSStore, input string, params map[string]string) (string, error) {
	query := getParam(params, "query", "")
	if query == "" {
		query = input
	}
	if query == "" {
		return "", fmt.Errorf("query is required for search operation")
	}
	scope := getParam(params, "scope", "")
	topK := parseIntSafe(getParam(params, "top_k", "10"), 10)
	entries, err := store.Search(query, scope, topK)
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal search results: %w", err)
	}
	return string(data), nil
}
