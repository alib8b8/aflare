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

// Package nodes re-exports LLM provider node types for backward
// compatibility with code that constructs them directly (e.g. tests and
// RegisterBuiltins). The actual implementations live in
// internal/nodes/providers/. Subpackage init() registration is
// centralized in register_all.go.
package nodes

import (
	"github.com/alib8b8/llm-box/internal/nodes/providers"
)

// Type aliases for backward compatibility with code/tests that construct
// these provider nodes directly by struct name (e.g. nodes_extra_test.go
// and RegisterBuiltins in node.go). Providers that register an
// OpenAI-compatible client directly (e.g. mistral, glm, qwen, kimi,
// deepseek, baichuan, internlm, yi, xverse, minimax, mimo, anthropic,
// gemini) do not need an alias because no caller references them by name.
type (
	OpenAINode  = providers.OpenAINode
	OllamaNode  = providers.OllamaNode
	CozeNode    = providers.CozeNode
	FastGPTNode = providers.FastGPTNode
	IMANode     = providers.IMANode
)
