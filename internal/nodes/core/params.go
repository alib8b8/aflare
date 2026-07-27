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

package core

import (
	"fmt"
	"strings"
	"unicode"
)

// GetParam returns params[key] if it exists and is non-empty, else defaultVal.
func GetParam(params map[string]string, key, defaultVal string) string {
	if v, ok := params[key]; ok && v != "" {
		return v
	}
	return defaultVal
}

// ParamInt safely parses an integer parameter with fallback default and optional bounds clamping.
// If parsing fails or the value is out of [min, max], the default is returned.
// Set min > max to disable clamping.
func ParamInt(params map[string]string, key string, defaultVal, min, max int) int {
	s := GetParam(params, key, "")
	if s == "" {
		return defaultVal
	}
	var v int
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil {
		return defaultVal
	}
	if min <= max {
		if v < min {
			return min
		}
		if v > max {
			return max
		}
	}
	return v
}

// ParamFloat safely parses a float parameter with fallback default and optional bounds clamping.
// If parsing fails or the value is out of [min, max], the default is returned.
// Set min > max to disable clamping.
func ParamFloat(params map[string]string, key string, defaultVal, min, max float64) float64 {
	s := GetParam(params, key, "")
	if s == "" {
		return defaultVal
	}
	var v float64
	if _, err := fmt.Sscanf(s, "%f", &v); err != nil {
		return defaultVal
	}
	if min <= max {
		if v < min {
			return min
		}
		if v > max {
			return max
		}
	}
	return v
}

// DefaultEndpointFor returns the default API endpoint URL for a known LLM
// provider. Falls back to the local Ollama endpoint for unknown providers.
func DefaultEndpointFor(provider string) string {
	switch provider {
	case "ollama":
		return "http://localhost:11434"
	case "openai":
		return "https://api.openai.com/v1"
	case "deepseek":
		return "https://api.deepseek.com/v1"
	case "glm":
		return "https://open.bigmodel.cn/api/paas/v4"
	case "kimi":
		return "https://api.moonshot.cn/v1"
	case "qwen":
		return "https://dashscope.aliyuncs.com/compatible-mode/v1"
	case "mistral":
		return "https://api.mistral.ai/v1"
	case "yi":
		return "https://api.lingyiwanwu.com/v1"
	case "anthropic":
		return "https://api.anthropic.com/v1"
	case "gemini":
		return "https://generativelanguage.googleapis.com/v1beta/openai"
	default:
		return "http://localhost:11434"
	}
}

// AgentTool describes a tool that a ReAct agent can invoke by delegating to
// another registered node.
type AgentTool struct {
	Name        string
	Description string
	NodeName    string
}

// ParseToolsList parses a comma-separated list of tool names (e.g.
// "fetch_url,json_parse") into a slice of AgentTool. Unknown names are
// dropped. If no names resolve, fetch_url and json_parse are returned as
// a safe default.
func ParseToolsList(toolsParam string) []AgentTool {
	toolMap := map[string]AgentTool{
		"fetch_url":        {Name: "fetch_url", Description: "Fetch content from a URL", NodeName: "fetch_url"},
		"http_request":     {Name: "http_request", Description: "Make HTTP requests with any method, headers, body", NodeName: "http_request"},
		"file_read":        {Name: "file_read", Description: "Read content from a file", NodeName: "file_read"},
		"file_write":       {Name: "file_write", Description: "Write content to a file", NodeName: "file_write"},
		"json_parse":       {Name: "json_parse", Description: "Parse and extract fields from JSON", NodeName: "json_parse"},
		"transform":        {Name: "transform", Description: "Transform text (uppercase, lowercase, trim, replace, regex)", NodeName: "transform"},
		"combine":          {Name: "combine", Description: "Combine multiple inputs into one", NodeName: "combine"},
		"template":         {Name: "template", Description: "Render Go template with variables", NodeName: "template_render"},
		"ollama":           {Name: "ollama_llm", Description: "Call Ollama LLM for analysis", NodeName: "ollama"},
		"code_interpreter": {Name: "code_interpreter", Description: "Execute Python code in a sandbox with file I/O", NodeName: "code_interpreter"},
		"execute":          {Name: "execute", Description: "Execute shell commands (disabled in safe mode)", NodeName: "execute"},
	}

	parts := strings.Split(toolsParam, ",")
	var tools []AgentTool
	for _, p := range parts {
		name := strings.TrimSpace(p)
		if tool, ok := toolMap[name]; ok {
			tools = append(tools, tool)
		}
	}
	if len(tools) == 0 {
		tools = append(tools, toolMap["fetch_url"])
		tools = append(tools, toolMap["json_parse"])
	}
	return tools
}

// BaseAgentParams returns the common parameter schema shared by all
// agent-style nodes (provider/model/api_key/endpoint).
func BaseAgentParams() []ParamSchema {
	return []ParamSchema{
		{Name: "provider", Type: "string", Description: "LLM provider (default: ollama)", Required: false, Default: "ollama"},
		{Name: "model", Type: "string", Description: "Model name (default: llama3)", Required: false, Default: "llama3"},
		{Name: "api_key", Type: "string", Description: "API key", Required: false},
		{Name: "endpoint", Type: "string", Description: "API endpoint URL", Required: false},
	}
}

// TitleCase capitalizes the first letter of s, leaving the rest unchanged.
// Replaces the deprecated strings.Title which doesn't handle Unicode properly.
func TitleCase(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
