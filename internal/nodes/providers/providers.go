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

// Package providers contains the LLM provider node implementations
// (OpenAI, Anthropic, Gemini, GLM, Qwen, Kimi, DeepSeek, Ollama, Mistral,
// MiniMax, Baichuan, InternLM, Yi, XVERSE, MiMo, Coze, FastGPT, IMA,
// SenseNova, AntLing, AndesGPT). Each provider registers itself with the
// global core.Registry via an init() function; subpackage registration is
// centralized by a blank import in internal/nodes/register_all.go.
//
// The shared infrastructure (Node interface, Registry, security helpers,
// LLM base client, parameter helpers) lives in internal/nodes/core and is
// referenced directly from here. A few lowercase parameter helpers are
// re-defined in this file to preserve the original call sites of the
// simulated provider nodes (sensenova/antling/andesgpt) that previously
// relied on helpers defined in the top-level nodes package.
package providers

import (
	"strconv"

	"github.com/alib8b8/llm-box/internal/nodes/core"
)

// getParam returns params[key] if it exists and is non-empty, else defaultVal.
// It is a thin wrapper around core.GetParam kept so that provider node files
// migrated from the top-level nodes package can keep their original call
// sites without referencing core directly.
func getParam(params map[string]string, key, defaultVal string) string {
	return core.GetParam(params, key, defaultVal)
}

// getMobileParam is an alias for getParam, kept for backward compatibility
// with provider node files that previously shared the helper from
// mobile_nodes.go.
func getMobileParam(params map[string]string, key, defaultVal string) string {
	return core.GetParam(params, key, defaultVal)
}

// parseIntSafe parses s as an int, returning defaultVal on error. Kept for
// backward compatibility with provider node files that used the
// mobile_nodes.go helper.
func parseIntSafe(s string, defaultVal int) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}
