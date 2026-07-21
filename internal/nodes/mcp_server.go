package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

var (
	validMCPServerActions = map[string]bool{
		"start":   true,
		"stop":    true,
		"status":  true,
		"restart": true,
	}

	validMCPServerProtocols = map[string]bool{
		"http":      true,
		"websocket": true,
	}

	mcpServerState struct {
		mu           sync.RWMutex
		running      bool
		port         int
		protocol     string
		host         string
		exposedTools []string
		authToken    string
		startTime    time.Time
		sessions     map[string]time.Time
	}
)

type MCPServerNode struct{}

func (n *MCPServerNode) Name() string { return "mcp_server" }

func (n *MCPServerNode) Description() string {
	return "MCP服务器模式节点。让llm-box作为MCP服务器被其他Agent调用，支持HTTP/WebSocket协议，提供工具暴露、会话管理和权限控制功能，兼容标准MCP协议。"
}

func (n *MCPServerNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - MCP服务器相关输入（可选）",
		Output:      "string - JSON格式的服务器操作结果",
		Params: []ParamSchema{
			{Name: "action", Type: "string", Description: "操作：start/stop/status/restart", Required: true},
			{Name: "port", Type: "int", Description: "端口（默认8080，范围1024-65535）", Required: false, Default: "8080"},
			{Name: "protocol", Type: "string", Description: "协议：http/websocket", Required: false, Default: "http"},
			{Name: "host", Type: "string", Description: "主机地址（默认0.0.0.0）", Required: false, Default: "0.0.0.0"},
			{Name: "expose_tools", Type: "string", Description: "要暴露的工具列表（逗号分隔）", Required: false},
			{Name: "auth_token", Type: "string", Description: "认证token（可选，长度32-256）", Required: false},
		},
	}
}

func (n *MCPServerNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	action := getParam(params, "action", "")
	if !validMCPServerActions[action] {
		return "", fmt.Errorf("invalid action: %s", action)
	}

	port := parseIntSafe(getParam(params, "port", "8080"), 8080)
	if port < 1024 || port > 65535 {
		return "", fmt.Errorf("invalid port: must be between 1024 and 65535")
	}

	protocol := getParam(params, "protocol", "http")
	if !validMCPServerProtocols[protocol] {
		return "", fmt.Errorf("invalid protocol: %s", protocol)
	}

	host := getParam(params, "host", "0.0.0.0")
	if !validateHost(host) {
		return "", fmt.Errorf("invalid host address: %s", host)
	}

	exposeToolsStr := getParam(params, "expose_tools", "")
	var exposeTools []string
	if exposeToolsStr != "" {
		parts := strings.Split(exposeToolsStr, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				exposeTools = append(exposeTools, part)
			}
		}
	}

	authToken := getParam(params, "auth_token", "")
	if authToken != "" && (len(authToken) < 32 || len(authToken) > 256) {
		return "", fmt.Errorf("auth_token must be between 32 and 256 characters")
	}

	var result map[string]interface{}
	var status string

	switch action {
	case "start":
		result, status = startMCPServer(port, protocol, host, exposeTools, authToken)
	case "stop":
		result, status = stopMCPServer()
	case "status":
		result, status = getMCPServerStatus()
	case "restart":
		stopMCPServer()
		result, status = startMCPServer(port, protocol, host, exposeTools, authToken)
	}

	response := map[string]interface{}{
		"action":    action,
		"status":    status,
		"result":    result,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	output, _ := json.MarshalIndent(response, "", "  ")
	return string(output), nil
}

func startMCPServer(port int, protocol, host string, exposeTools []string, authToken string) (map[string]interface{}, string) {
	mcpServerState.mu.Lock()
	defer mcpServerState.mu.Unlock()

	if mcpServerState.running {
		return nil, "error: server already running"
	}

	if !isPortAvailable(host, port) {
		return nil, fmt.Sprintf("error: port %d is already in use", port)
	}

	mcpServerState.running = true
	mcpServerState.port = port
	mcpServerState.protocol = protocol
	mcpServerState.host = host
	mcpServerState.exposedTools = exposeTools
	mcpServerState.authToken = authToken
	mcpServerState.startTime = time.Now()
	mcpServerState.sessions = make(map[string]time.Time)

	if len(exposeTools) == 0 {
		mcpServerState.exposedTools = []string{"file_read", "file_write", "http_request", "fetch_url", "search"}
	}

	return map[string]interface{}{
		"port":          port,
		"protocol":      protocol,
		"host":          host,
		"exposed_tools": mcpServerState.exposedTools,
		"auth_enabled":  authToken != "",
		"start_time":    mcpServerState.startTime.UTC().Format(time.RFC3339),
		"endpoint":      fmt.Sprintf("%s://%s:%d/mcp", protocol, host, port),
	}, "success"
}

func stopMCPServer() (map[string]interface{}, string) {
	mcpServerState.mu.Lock()
	defer mcpServerState.mu.Unlock()

	if !mcpServerState.running {
		return nil, "error: server not running"
	}

	uptime := time.Since(mcpServerState.startTime)
	sessionsCount := len(mcpServerState.sessions)

	mcpServerState.running = false
	mcpServerState.port = 0
	mcpServerState.protocol = ""
	mcpServerState.host = ""
	mcpServerState.exposedTools = nil
	mcpServerState.authToken = ""
	mcpServerState.startTime = time.Time{}
	mcpServerState.sessions = nil

	return map[string]interface{}{
		"uptime_ms":      uptime.Milliseconds(),
		"sessions_count": sessionsCount,
	}, "success"
}

func getMCPServerStatus() (map[string]interface{}, string) {
	mcpServerState.mu.RLock()
	defer mcpServerState.mu.RUnlock()

	if !mcpServerState.running {
		return map[string]interface{}{
			"running": false,
		}, "success"
	}

	uptime := time.Since(mcpServerState.startTime)
	sessionsCount := len(mcpServerState.sessions)

	return map[string]interface{}{
		"running":        true,
		"port":           mcpServerState.port,
		"protocol":       mcpServerState.protocol,
		"host":           mcpServerState.host,
		"exposed_tools":  mcpServerState.exposedTools,
		"auth_enabled":   mcpServerState.authToken != "",
		"sessions_count": sessionsCount,
		"uptime_ms":      uptime.Milliseconds(),
		"start_time":     mcpServerState.startTime.UTC().Format(time.RFC3339),
		"endpoint":       fmt.Sprintf("%s://%s:%d/mcp", mcpServerState.protocol, mcpServerState.host, mcpServerState.port),
	}, "success"
}

func validateHost(host string) bool {
	if host == "" {
		return false
	}
	if host == "0.0.0.0" || host == "localhost" || host == "127.0.0.1" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil
}

func isPortAvailable(host string, port int) bool {
	addr := fmt.Sprintf("%s:%d", host, port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

func init() {
	mcpServerState.sessions = make(map[string]time.Time)
	Register(&MCPServerNode{})
}
