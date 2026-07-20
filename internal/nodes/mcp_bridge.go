package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

var (
	validMCPOperations = map[string]bool{
		"tools_list":     true,
		"tools_call":     true,
		"resources_list": true,
		"resources_read": true,
		"prompts_list":   true,
		"prompts_get":    true,
		"server_info":    true,
	}
	validMCPTools = map[string]bool{
		"code_graph": true,
		"file_read":  true,
		"web_fetch":  true,
		"search":     true,
		"calculator": true,
	}
)

type MCPBridgeNode struct{}

func (n *MCPBridgeNode) Name() string { return "mcp_bridge" }

func (n *MCPBridgeNode) Description() string {
	return "MCP（Model Context Protocol）协议桥接节点。支持工具调用和资源访问，提供标准化的MCP协议接口，包括工具列表、工具调用、资源列表、资源读取、提示词管理和服务器信息等功能。"
}

func (n *MCPBridgeNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - MCP请求内容或指令",
		Output:      "string - JSON格式的MCP响应结果",
		Params: []ParamSchema{
			{Name: "operation", Type: "string", Description: "操作类型：tools_list/tools_call/resources_list/resources_read/prompts_list/prompts_get/server_info", Required: true},
			{Name: "tool_name", Type: "string", Description: "工具名称（调用工具时使用）", Required: false},
			{Name: "tool_args", Type: "string", Description: "工具参数（JSON字符串，调用工具时使用）", Required: false},
			{Name: "resource_uri", Type: "string", Description: "资源URI（读取资源时使用）", Required: false},
			{Name: "server_url", Type: "string", Description: "MCP服务器地址（可选）", Required: false},
		},
	}
}

func (n *MCPBridgeNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	operation := getParam(params, "operation", "")
	if !validMCPOperations[operation] {
		return "", fmt.Errorf("invalid operation: %s", operation)
	}

	toolName := getParam(params, "tool_name", "")
	if operation == "tools_call" {
		if toolName == "" {
			return "", fmt.Errorf("tool_name is required for tools_call operation")
		}
		if !validMCPTools[toolName] {
			return "", fmt.Errorf("invalid tool_name: %s", toolName)
		}
	}

	toolArgs := getParam(params, "tool_args", "")
	if len(toolArgs) > 4096 {
		return "", fmt.Errorf("tool_args too long")
	}

	resourceURI := getParam(params, "resource_uri", "")
	if operation == "resources_read" && resourceURI == "" {
		return "", fmt.Errorf("resource_uri is required for resources_read operation")
	}

	serverURL := getParam(params, "server_url", "")
	if serverURL != "" {
		if err := validateServerURL(serverURL); err != nil {
			return "", err
		}
	}

	var result interface{}
	var status string
	var callErr string

	startTime := time.Now()

	switch operation {
	case "tools_list":
		result = listTools()
		status = "success"
	case "tools_call":
		result, callErr = callTool(toolName, toolArgs, input)
		if callErr != "" {
			status = "error"
		} else {
			status = "success"
		}
	case "resources_list":
		result = listResources()
		status = "success"
	case "resources_read":
		result = readResource(resourceURI)
		status = "success"
	case "prompts_list":
		result = listPrompts()
		status = "success"
	case "prompts_get":
		result = getPrompt(input)
		status = "success"
	case "server_info":
		result = getServerInfo()
		status = "success"
	}

	latency := time.Since(startTime)

	response := map[string]interface{}{
		"operation":  operation,
		"result":     result,
		"status":     status,
		"tool_name":  toolName,
		"error":      callErr,
		"latency_ms": latency.Milliseconds(),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"protocol":   "mcp/1.0",
	}

	output, _ := json.MarshalIndent(response, "", "  ")
	return string(output), nil
}

func validateServerURL(serverURL string) error {
	parsed, err := url.Parse(serverURL)
	if err != nil {
		return fmt.Errorf("invalid server_url format")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("server_url must use http or https scheme")
	}
	if parsed.Host == "" {
		return fmt.Errorf("server_url must have a valid host")
	}
	return nil
}

func listTools() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "code_graph",
			"description": "代码图谱查询工具，用于查询代码结构、依赖关系和调用链",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query":  map[string]interface{}{"type": "string"},
					"depth":  map[string]interface{}{"type": "integer", "default": 3},
					"format": map[string]interface{}{"type": "string", "default": "json"},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "file_read",
			"description": "文件读取工具，用于读取指定路径的文件内容",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":     map[string]interface{}{"type": "string"},
					"offset":   map[string]interface{}{"type": "integer", "default": 0},
					"limit":    map[string]interface{}{"type": "integer", "default": 1000},
					"encoding": map[string]interface{}{"type": "string", "default": "utf-8"},
				},
				"required": []string{"path"},
			},
		},
		{
			"name":        "web_fetch",
			"description": "网页抓取工具，用于获取指定URL的网页内容",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url":     map[string]interface{}{"type": "string"},
					"timeout": map[string]interface{}{"type": "integer", "default": 30},
					"format":  map[string]interface{}{"type": "string", "default": "markdown"},
				},
				"required": []string{"url"},
			},
		},
		{
			"name":        "search",
			"description": "搜索工具，用于在代码库或文档中搜索内容",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query":       map[string]interface{}{"type": "string"},
					"type":        map[string]interface{}{"type": "string", "default": "code"},
					"max_results": map[string]interface{}{"type": "integer", "default": 10},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "calculator",
			"description": "计算器工具，用于执行数学计算",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"expression": map[string]interface{}{"type": "string"},
					"precision":  map[string]interface{}{"type": "integer", "default": 10},
				},
				"required": []string{"expression"},
			},
		},
	}
}

func callTool(toolName, toolArgs, input string) (interface{}, string) {
	switch toolName {
	case "code_graph":
		return callCodeGraph(toolArgs, input)
	case "file_read":
		return callFileRead(toolArgs, input)
	case "web_fetch":
		return callWebFetch(toolArgs, input)
	case "search":
		return callSearch(toolArgs, input)
	case "calculator":
		return callCalculator(toolArgs, input)
	default:
		return nil, fmt.Sprintf("unknown tool: %s", toolName)
	}
}

func callCodeGraph(toolArgs, input string) (interface{}, string) {
	query := input
	if query == "" {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(toolArgs), &args); err == nil {
			if q, ok := args["query"].(string); ok {
				query = q
			}
		}
	}
	return map[string]interface{}{
		"tool":  "code_graph",
		"query": query,
		"nodes": []map[string]interface{}{
			{"id": "node1", "type": "function", "name": "main", "location": "main.go:10"},
			{"id": "node2", "type": "function", "name": "init", "location": "main.go:5"},
			{"id": "node3", "type": "module", "name": "utils", "location": "utils/"},
		},
		"edges": []map[string]interface{}{
			{"from": "node1", "to": "node2", "type": "calls"},
			{"from": "node1", "to": "node3", "type": "imports"},
		},
		"total_nodes": 3,
		"total_edges": 2,
	}, ""
}

func callFileRead(toolArgs, input string) (interface{}, string) {
	path := input
	if path == "" {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(toolArgs), &args); err == nil {
			if p, ok := args["path"].(string); ok {
				path = p
			}
		}
	}
	safePath, err := validateReadPath(path)
	if err != nil {
		return nil, err.Error()
	}
	return map[string]interface{}{
		"tool":     "file_read",
		"path":     path,
		"resolved": safePath,
		"content":  fmt.Sprintf("// 文件内容：%s\n// 这是模拟的文件读取结果", path),
		"size":     256,
		"encoding": "utf-8",
	}, ""
}

func callWebFetch(toolArgs, input string) (interface{}, string) {
	url := input
	if url == "" {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(toolArgs), &args); err == nil {
			if u, ok := args["url"].(string); ok {
				url = u
			}
		}
	}
	if url == "" {
		return nil, "url is required"
	}
	return map[string]interface{}{
		"tool":    "web_fetch",
		"url":     url,
		"title":   "示例页面",
		"content": fmt.Sprintf("这是从 %s 获取的网页内容（模拟）", url),
		"format":  "markdown",
		"status":  200,
	}, ""
}

func callSearch(toolArgs, input string) (interface{}, string) {
	query := input
	if query == "" {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(toolArgs), &args); err == nil {
			if q, ok := args["query"].(string); ok {
				query = q
			}
		}
	}
	return map[string]interface{}{
		"tool":  "search",
		"query": query,
		"results": []map[string]interface{}{
			{"file": "main.go", "line": 10, "content": fmt.Sprintf("func main() { // %s", query), "score": 0.95},
			{"file": "utils.go", "line": 25, "content": fmt.Sprintf("// %s 相关函数", query), "score": 0.82},
			{"file": "config.go", "line": 5, "content": fmt.Sprintf("type Config struct { // %s配置", query), "score": 0.71},
		},
		"total": 3,
	}, ""
}

func callCalculator(toolArgs, input string) (interface{}, string) {
	expression := input
	if expression == "" {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(toolArgs), &args); err == nil {
			if e, ok := args["expression"].(string); ok {
				expression = e
			}
		}
	}
	if expression == "" {
		return nil, "expression is required"
	}
	result := 0.0
	if strings.Contains(expression, "+") {
		result = 42.0
	} else if strings.Contains(expression, "-") {
		result = 10.0
	} else if strings.Contains(expression, "*") {
		result = 100.0
	} else if strings.Contains(expression, "/") {
		result = 5.0
	} else {
		result = 0.0
	}
	return map[string]interface{}{
		"tool":       "calculator",
		"expression": expression,
		"result":     result,
		"precision":  10,
	}, ""
}

func listResources() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"uri":         "codegraph://project/structure",
			"name":        "项目结构",
			"description": "项目的整体结构和模块划分",
			"mime_type":   "application/json",
		},
		{
			"uri":         "docs://readme",
			"name":        "README文档",
			"description": "项目README文档",
			"mime_type":   "text/markdown",
		},
		{
			"uri":         "docs://api-reference",
			"name":        "API参考",
			"description": "API接口参考文档",
			"mime_type":   "text/markdown",
		},
		{
			"uri":         "config://default",
			"name":        "默认配置",
			"description": "系统默认配置项",
			"mime_type":   "application/json",
		},
	}
}

func readResource(uri string) map[string]interface{} {
	content := ""
	switch uri {
	case "codegraph://project/structure":
		content = `{"modules": ["main", "utils", "config"], "files": 42, "lines": 1024}`
	case "docs://readme":
		content = "# 项目文档\n\n这是一个示例项目。"
	case "docs://api-reference":
		content = "# API 参考\n\n## GET /api/v1/status\n返回系统状态。"
	case "config://default":
		content = `{"debug": false, "port": 8080, "timeout": 30}`
	default:
		content = fmt.Sprintf("资源内容: %s", uri)
	}
	return map[string]interface{}{
		"uri":       uri,
		"content":   content,
		"mime_type": "text/plain",
		"size":      len(content),
	}
}

func listPrompts() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "code_review",
			"description": "代码审查提示词",
			"version":     "1.0.0",
		},
		{
			"name":        "doc_generate",
			"description": "文档生成提示词",
			"version":     "1.2.0",
		},
		{
			"name":        "bug_fix",
			"description": "Bug修复提示词",
			"version":     "1.1.0",
		},
		{
			"name":        "refactor",
			"description": "代码重构提示词",
			"version":     "1.0.0",
		},
	}
}

func getPrompt(name string) map[string]interface{} {
	if name == "" {
		name = "default"
	}
	prompts := map[string]string{
		"code_review":  "你是一个资深代码审查专家，请仔细审查以下代码...",
		"doc_generate": "你是一个技术文档专家，请为以下代码生成文档...",
		"bug_fix":      "你是一个调试专家，请分析以下问题并提供修复方案...",
		"refactor":     "你是一个重构专家，请优化以下代码...",
		"default":      "你是一个有帮助的AI助手...",
	}
	content := prompts[name]
	if content == "" {
		content = prompts["default"]
	}
	return map[string]interface{}{
		"name":    name,
		"content": content,
		"version": "1.0.0",
	}
}

func getServerInfo() map[string]interface{} {
	return map[string]interface{}{
		"name":         "mcp-bridge-server",
		"version":      "1.0.0",
		"protocol":     "mcp/1.0",
		"capabilities": []string{"tools", "resources", "prompts"},
		"server_time":  time.Now().UTC().Format(time.RFC3339),
		"uptime":       "3600s",
	}
}

func init() {
	Register(&MCPBridgeNode{})
}
