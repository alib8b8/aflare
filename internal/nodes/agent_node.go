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
	"fmt"
	"math"
	"strconv"

	"github.com/alib8b8/llm-box/internal/nodes/core"
)

// AgentNode is the canonical "agent" node that runs a ReAct loop with
// tool use. The parameter helpers (getParam/paramInt/paramFloat), the
// default-endpoint lookup, and the tool-list parser have been moved to
// internal/nodes/core/params.go so that sub-packages under internal/nodes
// can share them without creating import cycles. The lowercase wrappers
// below preserve backward compatibility for the existing node files in
// this package.

// getParam returns params[key] if it exists and is non-empty, else defaultVal.
func getParam(params map[string]string, key, defaultVal string) string {
	return core.GetParam(params, key, defaultVal)
}

// paramInt safely parses an integer parameter with fallback default and
// optional bounds clamping. Set min > max to disable clamping.
func paramInt(params map[string]string, key string, defaultVal, min, max int) int {
	return core.ParamInt(params, key, defaultVal, min, max)
}

// paramFloat safely parses a float parameter with fallback default and
// optional bounds clamping. Set min > max to disable clamping.
func paramFloat(params map[string]string, key string, defaultVal, min, max float64) float64 {
	return core.ParamFloat(params, key, defaultVal, min, max)
}

// defaultEndpointFor returns the default API endpoint for a known provider.
func defaultEndpointFor(provider string) string {
	return core.DefaultEndpointFor(provider)
}

// getMobileParam returns params[key] if it exists and is non-empty, else
// defaultVal. Equivalent to core.GetParam; kept for backward compatibility
// with node files that previously shared the helper from mobile_nodes.go.
func getMobileParam(params map[string]string, key, defaultVal string) string {
	return core.GetParam(params, key, defaultVal)
}

// parseIntSafe parses s as an int, returning defaultVal on error. Kept for
// backward compatibility with node files that used the mobile_nodes.go helper.
func parseIntSafe(s string, defaultVal int) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}

// parseFloatSafe parses s as a float64, returning defaultVal on error or
// NaN/Inf. Kept for backward compatibility with node files that used the
// mobile_nodes.go helper.
func parseFloatSafe(s string, defaultVal float64) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return defaultVal
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return defaultVal
	}
	return v
}

// truncateInput truncates s to at most maxLen characters (by rune), appending
// "..." if truncation occurred. Kept for backward compatibility with node files
// that used the mobile_nodes.go helper.
func truncateInput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	r := []rune(s)
	if len(r) <= maxLen {
		return s[:maxLen]
	}
	return string(r[:maxLen]) + "..."
}

// AgentTool describes a tool that a ReAct agent can invoke.
type AgentTool = core.AgentTool

// parseToolsList parses a comma-separated list of tool names into AgentTools.
func parseToolsList(toolsParam string) []AgentTool {
	return core.ParseToolsList(toolsParam)
}

// baseAgentParams returns the common parameter schema shared by agent nodes.
func baseAgentParams() []ParamSchema {
	return core.BaseAgentParams()
}

type AgentNode struct{}

func init() {
	Register(&AgentNode{})
}

func (n *AgentNode) Name() string {
	return "agent"
}

func (n *AgentNode) Description() string {
	return "Run an autonomous agent with ReAct loop and tool use"
}

func (n *AgentNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "agent",
		Description: "Autonomous agent node with ReAct reasoning loop and tool use capabilities",
		Input:       "string - the task or question for the agent",
		Output:      "string - the agent's final answer",
		Params: []ParamSchema{
			{Name: "provider", Type: "string", Description: "LLM provider: ollama, openai, deepseek, glm, kimi, qwen, mistral, yi (default: ollama)", Required: false, Default: "ollama"},
			{Name: "model", Type: "string", Description: "Model name (default: llama3)", Required: false, Default: "llama3"},
			{Name: "api_key", Type: "string", Description: "API key (for cloud providers)", Required: false},
			{Name: "endpoint", Type: "string", Description: "API endpoint URL", Required: false},
			{Name: "system", Type: "string", Description: "System prompt / role definition for the agent", Required: false},
			{Name: "tools", Type: "string", Description: "Comma-separated list of tools to enable: fetch_url,http_request,file_read,file_write,json_parse,transform,combine,ollama,openai,code_interpreter,execute", Required: false, Default: "fetch_url,json_parse"},
			{Name: "max_iterations", Type: "string", Description: "Maximum number of ReAct iterations (default: 10)", Required: false, Default: "10"},
			{Name: "enable_thinking", Type: "string", Description: "Enable deep thinking / chain-of-thought mode (default: false)", Required: false, Default: "false"},
			{Name: "show_thinking", Type: "string", Description: "Show the thinking chain in output (default: true)", Required: false, Default: "true"},
		},
	}
}

func (n *AgentNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	provider := getParam(params, "provider", "ollama")
	model := getParam(params, "model", "llama3")
	apiKey := getParam(params, "api_key", "")
	endpoint := getParam(params, "endpoint", defaultEndpointFor(provider))
	systemPrompt := getParam(params, "system", "")
	toolsParam := getParam(params, "tools", "fetch_url,json_parse")
	maxItersStr := getParam(params, "max_iterations", "10")
	enableThinking := getParam(params, "enable_thinking", "false") == "true"
	showThinking := getParam(params, "show_thinking", "true") == "true"

	maxIters := 10
	if _, err := fmt.Sscanf(maxItersStr, "%d", &maxIters); err != nil {
		// keep default value on parse failure
	}
	if maxIters < 1 {
		maxIters = 1
	}
	if maxIters > 50 {
		maxIters = 50
	}

	tools := parseToolsList(toolsParam)

	reg := GetGlobalRegistry()

	agent := NewReActAgent(provider, model, apiKey, endpoint, systemPrompt, maxIters, tools, reg, enableThinking, showThinking)
	return agent.Run(ctx, input)
}
