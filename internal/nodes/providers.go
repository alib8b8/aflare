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

// Package nodes re-exports LLM provider node types for backward
// compatibility with code that constructs them directly (e.g. tests and
// RegisterBuiltins). The actual implementations live in
// internal/nodes/providers/. Subpackage init() registration is
// centralized in register_all.go.
package nodes

import (
	"github.com/alib8b8/aflare/internal/nodes/providers"
)

// Type aliases for backward compatibility with code/tests that construct
// these provider nodes directly by struct name (e.g. nodes_extra_test.go
// and RegisterBuiltins in node.go). The OpenAI-compatible providers
// (openai, anthropic, gemini, glm, kimi, qwen, deepseek, mistral, yi,
// baichuan, internlm, minimax, xverse, mimo, coze, ima) no longer have
// dedicated struct types — they are registered from a single config
// table in providers/openai_compatible.go — so they have no alias here.
// Only providers with bespoke implementations keep a struct alias.
type (
	OllamaNode  = providers.OllamaNode
	FastGPTNode = providers.FastGPTNode
)
