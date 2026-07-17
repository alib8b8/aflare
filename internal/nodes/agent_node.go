package nodes

import (
	"context"
	"fmt"
	"strings"
)

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
	fmt.Sscanf(maxItersStr, "%d", &maxIters)
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

func defaultEndpointFor(provider string) string {
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
	default:
		return "http://localhost:11434"
	}
}

func parseToolsList(toolsParam string) []AgentTool {
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

func getParam(params map[string]string, key, defaultVal string) string {
	if v, ok := params[key]; ok && v != "" {
		return v
	}
	return defaultVal
}
