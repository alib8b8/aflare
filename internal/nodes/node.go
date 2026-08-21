// Copyright (c) 2026 aflare Contributors
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

// Package nodes provides the workflow node registry and the built-in node
// implementations. The shared infrastructure (Node interface, Registry,
// security helpers, LLM base client, parameter helpers) lives in the
// internal/nodes/core sub-package and is re-exported here for backward
// compatibility with existing callers.
package nodes

import (
	"context"

	"github.com/alib8b8/aflare/internal/nodes/core"
	"github.com/alib8b8/aflare/internal/nodes/providers"
)

// Re-exports of core types via aliases so existing callers can keep using
// nodes.Node, nodes.Registry, nodes.NodeSchema, etc. without modification.
type (
	ParamSchema      = core.ParamSchema
	NodeSchema       = core.NodeSchema
	Node             = core.Node
	StreamingNode    = core.StreamingNode
	NodeMetadata     = core.NodeMetadata
	ExternalNode     = core.ExternalNode
	NodeExecStats    = core.NodeExecStats
	Registry         = core.Registry
	NodeInfo         = core.NodeInfo
	LLMCallTelemetry = core.LLMCallTelemetry
	LLMCallSink      = core.LLMCallSink
	LLMUsage         = core.LLMUsage
)

// WithLLMCallSink re-exports core.WithLLMCallSink so callers in packages
// that already depend on nodes (not core) can attach a sink to a context.
func WithLLMCallSink(ctx context.Context, sink LLMCallSink) context.Context {
	return core.WithLLMCallSink(ctx, sink)
}

// LLMCallSinkFrom re-exports core.LLMCallSinkFrom.
func LLMCallSinkFrom(ctx context.Context) LLMCallSink {
	return core.LLMCallSinkFrom(ctx)
}

// NewExternalNode creates a new ExternalNode.
func NewExternalNode(metadata NodeMetadata, nodePath string) *ExternalNode {
	return core.NewExternalNode(metadata, nodePath)
}

// NewRegistry creates a new registry.
func NewRegistry() *Registry {
	return core.NewRegistry()
}

// Register adds a node to the global registry.
func Register(node Node) {
	core.Register(node)
}

// Get retrieves a node from the global registry by name.
func Get(name string) (Node, bool) {
	return core.Get(name)
}

// List returns all registered node names from the global registry.
func List() []string {
	return core.List()
}

// SetSafeMode sets safe mode on the global registry.
func SetSafeMode(enabled bool) {
	core.SetSafeMode(enabled)
}

// IsSafeMode returns whether safe mode is enabled on the global registry.
func IsSafeMode() bool {
	return core.IsSafeMode()
}

// GetGlobalRegistry returns the global registry.
func GetGlobalRegistry() *Registry {
	return core.GetGlobalRegistry()
}

// LoadExternalNodes loads external nodes into the global registry.
func LoadExternalNodes(dir string) error {
	return core.LoadExternalNodes(dir)
}

// RegisterBuiltins registers all built-in nodes to a registry. This is
// kept in the nodes package (rather than core) because it references the
// concrete node structs defined in this package and its sub-packages.
func RegisterBuiltins(reg *Registry) {
	reg.Register(&ConditionNode{})
	reg.Register(&FetchURLNode{})
	reg.Register(&HTTPRequestNode{})
	reg.Register(&ExecuteNode{})
	reg.Register(&TemplateRenderNode{})
	reg.Register(&FileWriteNode{})
	reg.Register(&FileReadNode{})
	reg.Register(&FastGPTNode{})
	reg.Register(&JSONParseNode{})
	reg.Register(&CombineNode{})
	reg.Register(&TransformNode{})
	reg.Register(&NotifyNode{})
	reg.Register(&OllamaNode{})
	reg.Register(&CallNode{})

	// Data, knowledge and document nodes. These also self-register via
	// init() into the global registry, but RegisterBuiltins is invoked on
	// freshly created registries (e.g. `aflare list`, MCP servers) that
	// do not share the global one, so they must be registered here too.
	// Register is idempotent on the global registry.
	reg.Register(&SQLQueryNode{})
	reg.Register(&RAGNode{})
	reg.Register(&KnowledgeGraphNode{})
	reg.Register(&CodeKnowledgeGraphNode{})
	reg.Register(&OfficeNode{})
	reg.Register(&DocParseNode{})
	reg.Register(&DocGenNode{})
	reg.Register(&MultimodalNode{})
	reg.Register(&VerifyNode{})
	reg.Register(&ClarifyNode{})
	reg.Register(&PreferenceNode{})
	reg.Register(&SkillDistillNode{})
	reg.Register(&EngineerSkillsNode{})
	reg.Register(&CompressNode{})
	reg.Register(&CodexAgentNode{})

	// OpenAI-compatible providers (openai, glm, kimi, qwen, deepseek,
	// anthropic, gemini, mistral, yi, baichuan, internlm, minimax,
	// xverse, mimo, coze, ima) are registered from the consolidated
	// config table in providers/openai_compatible.go so that this local
	// registry gets real, functional nodes rather than zero-value stubs.
	for _, cfg := range providers.OpenAICompatibleConfigs() {
		reg.Register(core.NewOpenAICompatibleNode(cfg))
	}
}
