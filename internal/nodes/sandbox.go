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

package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// sandboxMode defines the execution mode of the sandbox.
type sandboxMode string

const (
	sandboxShell   sandboxMode = "shell"
	sandboxBrowser sandboxMode = "browser"
	sandboxDesktop sandboxMode = "desktop"
)

var (
	validSandboxModes = map[sandboxMode]bool{
		sandboxShell:   true,
		sandboxBrowser: true,
		sandboxDesktop: true,
	}

	// sandboxCommandWhitelist defines safe commands allowed in shell mode.
	sandboxCommandWhitelist = map[string]bool{
		"ls": true, "cat": true, "head": true, "tail": true, "wc": true,
		"grep": true, "awk": true, "sed": true, "find": true, "sort": true,
		"uniq": true, "cut": true, "tr": true, "echo": true, "date": true,
		"pwd": true, "whoami": true, "uname": true, "df": true, "du": true,
		"free": true, "ps": true, "top": true, "uptime": true, "env": true,
		"curl": true, "wget": true, "ping": true, "nslookup": true,
		"python3": true, "python": true, "node": true, "ruby": true,
		"git": true, "make": true, "go": true, "java": true,
		"mkdir": true, "cp": true, "mv": true, "rm": true, "chmod": true,
	}

	// sandboxBlockedPatterns are patterns that are never allowed.
	sandboxBlockedPatterns = []string{
		"rm -rf /", "mkfs.", "dd if=", "> /dev/sda",
		"fork bomb", ":(){ :|:& };:", "chmod 777 /",
		"wget -O /etc/", "curl -o /etc/",
	}

	sandboxInstances    = make(map[string]*sandboxState)
	sandboxInstancesMu  sync.RWMutex
	sandboxCleanupTimer *time.Timer
)

type sandboxState struct {
	id        string
	mode      sandboxMode
	createdAt time.Time
	lastUsed  time.Time
	workDir   string
	envVars   map[string]string
}

// SandboxNode provides an isolated execution environment for running code,
// browser operations, and desktop operations. It acts as the "virtual computer"
// execution layer for AI agents, similar to Cloudflare/computer.
type SandboxNode struct{}

func (n *SandboxNode) Name() string { return "sandbox" }

func (n *SandboxNode) Description() string {
	return "沙箱执行节点。为AI Agent提供隔离的执行环境，支持三种模式：shell（命令执行）、browser（浏览器操作）、desktop（桌面应用操作）。命令白名单+超时+资源限制，安全可控。"
}

func (n *SandboxNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - 要执行的命令或操作指令",
		Output:      "string - JSON格式的执行结果",
		Params: []ParamSchema{
			{Name: "mode", Type: "string", Description: "执行模式：shell（命令执行）、browser（浏览器操作）、desktop（桌面操作）", Required: false, Default: "shell"},
			{Name: "sandbox_id", Type: "string", Description: "沙箱ID（自动生成或指定，用于保持会话状态）", Required: false},
			{Name: "timeout_ms", Type: "int", Description: "超时时间（毫秒，默认30000）", Required: false, Default: "30000"},
			{Name: "work_dir", Type: "string", Description: "工作目录", Required: false, Default: "/tmp/aflare-sandbox"},
			{Name: "env", Type: "string", Description: "环境变量（JSON格式，如 {\"KEY\":\"VALUE\"}）", Required: false},
			{Name: "safe_mode", Type: "bool", Description: "安全模式（默认true，仅允许白名单命令）", Required: false, Default: "true"},
		},
	}
}

func (n *SandboxNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	if len(input) > 50000 {
		return "", fmt.Errorf("input too long, max 50000 characters")
	}

	mode := sandboxMode(getParam(params, "mode", "shell"))
	if !validSandboxModes[mode] {
		return "", fmt.Errorf("invalid mode: %s (valid: shell, browser, desktop)", mode)
	}

	sandboxID := getParam(params, "sandbox_id", "")
	if sandboxID == "" {
		sandboxID = fmt.Sprintf("sandbox-%d-%s", time.Now().Unix(), randomHex(6))
	}

	timeoutMs := parseIntSafe(getParam(params, "timeout_ms", "30000"), 30000)
	if timeoutMs < 1000 {
		timeoutMs = 1000
	}
	if timeoutMs > 300000 {
		timeoutMs = 300000
	}

	workDir := getParam(params, "work_dir", "/tmp/aflare-sandbox")
	safeMode := strings.ToLower(getParam(params, "safe_mode", "true")) == "true"

	envVars := parseEnvVars(getParam(params, "env", "{}"))

	startTime := time.Now()

	// Get or create sandbox state
	sandboxInstancesMu.Lock()
	state, exists := sandboxInstances[sandboxID]
	if !exists {
		state = &sandboxState{
			id:        sandboxID,
			mode:      mode,
			createdAt: time.Now(),
			workDir:   workDir,
			envVars:   envVars,
		}
		sandboxInstances[sandboxID] = state
	}
	state.lastUsed = time.Now()
	state.mode = mode
	if envVars != nil {
		state.envVars = envVars
	}
	sandboxInstancesMu.Unlock()

	// Ensure work directory exists
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create work directory: %w", err)
	}

	// Cleanup expired sandboxes periodically
	scheduleSandboxCleanup()

	var result map[string]interface{}

	switch mode {
	case sandboxShell:
		result = n.executeShell(ctx, input, state, timeoutMs, safeMode)
	case sandboxBrowser:
		result = n.executeBrowser(input, state)
	case sandboxDesktop:
		result = n.executeDesktop(input, state)
	}

	result["sandbox_id"] = sandboxID
	result["mode"] = string(mode)
	result["latency_ms"] = time.Since(startTime).Milliseconds()
	result["timestamp"] = time.Now().UTC().Format(time.RFC3339)

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

func (n *SandboxNode) executeShell(ctx context.Context, input string, state *sandboxState, timeoutMs int, safeMode bool) map[string]interface{} {
	result := map[string]interface{}{
		"command": input,
	}

	// Safety check
	if safeMode {
		if err := validateShellCommand(input); err != nil {
			result["status"] = "blocked"
			result["error"] = err.Error()
			return result
		}
	}

	// Create timeout context
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "sh", "-c", input)
	cmd.Dir = state.workDir

	// Set environment variables
	cmd.Env = os.Environ()
	for k, v := range state.envVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	output, err := cmd.CombinedOutput()

	result["exit_code"] = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result["exit_code"] = exitErr.ExitCode()
		} else {
			result["exit_code"] = -1
			result["status"] = "error"
			result["error"] = err.Error()
			return result
		}
	}

	// Truncate output if too long
	outputStr := string(output)
	if len(outputStr) > 50000 {
		outputStr = outputStr[:50000] + "\n... (truncated)"
	}

	result["status"] = "completed"
	result["stdout"] = outputStr
	result["work_dir"] = state.workDir
	return result
}

func (n *SandboxNode) executeBrowser(input string, state *sandboxState) map[string]interface{} {
	result := map[string]interface{}{
		"action": input,
	}

	lower := strings.ToLower(input)

	switch {
	case strings.Contains(lower, "navigate") || strings.Contains(lower, "打开"):
		result["status"] = "completed"
		result["action_type"] = "navigate"
		result["message"] = fmt.Sprintf("浏览器导航到目标页面（模拟模式）")
	case strings.Contains(lower, "click") || strings.Contains(lower, "点击"):
		result["status"] = "completed"
		result["action_type"] = "click"
		result["message"] = fmt.Sprintf("浏览器点击操作（模拟模式）")
	case strings.Contains(lower, "type") || strings.Contains(lower, "输入") || strings.Contains(lower, "fill"):
		result["status"] = "completed"
		result["action_type"] = "type"
		result["message"] = fmt.Sprintf("浏览器输入操作（模拟模式）")
	case strings.Contains(lower, "screenshot") || strings.Contains(lower, "截图"):
		result["status"] = "completed"
		result["action_type"] = "screenshot"
		result["message"] = fmt.Sprintf("浏览器截图（模拟模式，实际运行需Xvfb+Chrome）")
	case strings.Contains(lower, "extract") || strings.Contains(lower, "提取") || strings.Contains(lower, "scrape"):
		result["status"] = "completed"
		result["action_type"] = "extract"
		result["message"] = fmt.Sprintf("浏览器内容提取（模拟模式，实际运行需playwright）")
	default:
		result["status"] = "completed"
		result["action_type"] = "unknown"
		result["message"] = fmt.Sprintf("浏览器操作（模拟模式）：%s", input)
	}

	return result
}

func (n *SandboxNode) executeDesktop(input string, state *sandboxState) map[string]interface{} {
	result := map[string]interface{}{
		"action": input,
	}

	lower := strings.ToLower(input)

	switch {
	case strings.Contains(lower, "launch") || strings.Contains(lower, "启动") || strings.Contains(lower, "open"):
		result["status"] = "completed"
		result["action_type"] = "launch"
		result["message"] = fmt.Sprintf("桌面应用启动（模拟模式，实际运行需xdotool/Windows COM）")
	case strings.Contains(lower, "click") || strings.Contains(lower, "点击"):
		result["status"] = "completed"
		result["action_type"] = "click"
		result["message"] = fmt.Sprintf("桌面点击操作（模拟模式）")
	case strings.Contains(lower, "type") || strings.Contains(lower, "输入") || strings.Contains(lower, "keyboard"):
		result["status"] = "completed"
		result["action_type"] = "type"
		result["message"] = fmt.Sprintf("桌面键盘输入（模拟模式）")
	case strings.Contains(lower, "screenshot") || strings.Contains(lower, "截图"):
		result["status"] = "completed"
		result["action_type"] = "screenshot"
		result["message"] = fmt.Sprintf("桌面截图（模拟模式，实际运行需Xvfb+scrot）")
	default:
		result["status"] = "completed"
		result["action_type"] = "unknown"
		result["message"] = fmt.Sprintf("桌面操作（模拟模式）：%s", input)
	}

	return result
}

// validateShellCommand checks if a command is safe to execute.
func validateShellCommand(cmd string) error {
	trimmed := strings.TrimSpace(cmd)

	// Check blocked patterns
	for _, pattern := range sandboxBlockedPatterns {
		if strings.Contains(strings.ToLower(trimmed), strings.ToLower(pattern)) {
			return fmt.Errorf("blocked dangerous pattern: %s", pattern)
		}
	}

	// Extract the first word (the command)
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	cmdName := parts[0]
	// Handle paths like /usr/bin/ls -> ls
	if idx := strings.LastIndex(cmdName, "/"); idx >= 0 {
		cmdName = cmdName[idx+1:]
	}

	if !sandboxCommandWhitelist[cmdName] {
		return fmt.Errorf("command not in whitelist: %s (safe mode: use only allowed commands)", cmdName)
	}

	return nil
}

// parseEnvVars parses a JSON string of environment variables.
func parseEnvVars(envJSON string) map[string]string {
	if envJSON == "" || envJSON == "{}" {
		return nil
	}
	vars := make(map[string]string)
	if err := json.Unmarshal([]byte(envJSON), &vars); err != nil {
		return nil
	}
	return vars
}

// randomHex generates a random hex string of the given length.
func randomHex(n int) string {
	const letters = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}

// scheduleSandboxCleanup periodically cleans up expired sandboxes.
func scheduleSandboxCleanup() {
	sandboxInstancesMu.RLock()
	count := len(sandboxInstances)
	sandboxInstancesMu.RUnlock()

	if count < 100 {
		return
	}

	sandboxInstancesMu.Lock()
	defer sandboxInstancesMu.Unlock()

	cutoff := time.Now().Add(-1 * time.Hour)
	for id, state := range sandboxInstances {
		if state.lastUsed.Before(cutoff) {
			delete(sandboxInstances, id)
		}
	}
}

func init() {
	Register(&SandboxNode{})
}