// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​​‌​‌​​​​​‌​‌​‌‌‌​​‌​​‌‌​​‌‌​‌‌‌​​‌‌​‌​‌‌​​​​​‌​​​​​​​​​​​​​​​​​​​‌​​​‌​‌​​‌​​‌⁠
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

// LLM base types and the OpenAICompatibleNode re-exported from
// internal/nodes/core. The actual implementation lives in core/llm_base.go
// so that sub-packages under internal/nodes/ can construct
// OpenAI-compatible nodes without creating import cycles.
package nodes

import (
	"github.com/alib8b8/aflare/internal/nodes/core"
)

// Aliases for the LLM base types so existing callers in the nodes package
// and external importers can keep using the unqualified names.
type (
	LLMMessage    = core.LLMMessage
	LLMRequest    = core.LLMRequest
	LLMChoice     = core.LLMChoice
	LLMResponse   = core.LLMResponse
	LLMNodeConfig = core.LLMNodeConfig

	OpenAICompatibleNode = core.OpenAICompatibleNode
)

// NewOpenAICompatibleNode constructs an OpenAICompatibleNode from config.
func NewOpenAICompatibleNode(config LLMNodeConfig) *OpenAICompatibleNode {
	return core.NewOpenAICompatibleNode(config)
}
