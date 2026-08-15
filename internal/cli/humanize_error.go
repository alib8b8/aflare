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

package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// humanizeError translates a raw Go/node error into a user-friendly message
// (断点11). It returns:
//   - human: a short, actionable Chinese message the user sees on the terminal.
//   - debug: the original error text, preserved for debug logging.
//
// When no translation matches, human falls back to the original error string
// so no information is lost.
func humanizeError(err error, nodeName string) (human, debug string) {
	if err == nil {
		return "", ""
	}
	debug = err.Error()

	// 1. Structured / typed errors first (most reliable).
	if h := humanizeTypedError(err); h != "" {
		return h, debug
	}

	// 2. Pattern-based matching on the error string.
	if h := humanizeByPattern(err.Error(), nodeName); h != "" {
		return h, debug
	}

	// 3. Fallback: return the original error so nothing is lost.
	return debug, debug
}

// humanizeTypedError inspects well-known Go error types (fs.PathError,
// net.DNSError, exec.Error, net.OpError) to produce precise translations.
func humanizeTypedError(err error) string {
	// File not found / permission denied.
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		path := pathErr.Path
		switch {
		case errors.Is(pathErr.Err, fs.ErrNotExist):
			return fmt.Sprintf("文件不存在：%s。检查路径是否正确。", path)
		case errors.Is(pathErr.Err, fs.ErrPermission):
			return fmt.Sprintf("权限不足，无法访问 %s。检查文件权限。", path)
		default:
			return fmt.Sprintf("文件操作失败：%s（%v）。", path, pathErr.Err)
		}
	}

	// DNS / network lookup failure.
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return fmt.Sprintf("网络错误：无法解析域名 %s。检查网络连接或 URL 是否正确。", dnsErr.Name)
	}

	// Command not found (exec package).
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return fmt.Sprintf("未找到命令：%s。请先安装对应工具。", execErr.Name)
	}

	// Network operation errors (connection refused, timeout, etc.).
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Timeout() {
			return "请求超时。可以在工作流中增加 timeout 参数。"
		}
		if strings.Contains(strings.ToLower(opErr.Err.Error()), "refused") {
			return "目标服务拒绝连接。检查服务是否运行、端口是否正确。"
		}
		return fmt.Sprintf("网络错误：%v。检查网络连接或目标地址是否正确。", opErr.Err)
	}

	return ""
}

// humanizeByPattern matches common error-string patterns that don't expose
// a typed error (e.g. fmt.Errorf-wrapped HTTP errors from nodes).
func humanizeByPattern(errStr, nodeName string) string {
	lower := strings.ToLower(errStr)

	// --- LLM-specific errors (check before generic network patterns so
	// "ollama not running...connection refused" matches the LLM rule, not
	// the generic connection-refused rule) ---
	if strings.Contains(lower, "ollama not running") || strings.Contains(lower, "ollama returned status") {
		return "Ollama 服务未运行或不可用。请检查：\n  1. Ollama 是否运行：ollama list\n  2. 模型是否已拉取：ollama pull llama3"
	}
	if matchPattern(lower, `api key required`, `api_key.*required`) {
		return "缺少 API Key。运行 aflare init 配置 LLM，或设置对应的环境变量。"
	}

	// --- Node-not-found ---
	if strings.Contains(lower, "not found in registry") {
		node := extractFirstGroup(errStr, `node\s+'([^']+)'`)
		if node == "" {
			node = extractFirstGroup(errStr, `node\s+"([^"]+)"`)
		}
		if node != "" {
			return fmt.Sprintf("未知节点类型：%s。检查工作流 YAML 中 node 字段是否拼写正确。", node)
		}
		return "未知节点类型。检查工作流 YAML 中 node 字段是否拼写正确。"
	}

	// --- Network errors ---
	if matchPattern(lower, `dial tcp.*lookup.*no such host`, `dial tcp.*no such host`) {
		host := extractFirstGroup(errStr, `lookup\s+([^\s:]+)`)
		if host == "" {
			host = extractFirstGroup(errStr, `dial tcp.*?([^\s:/]+)`)
		}
		if host != "" {
			return fmt.Sprintf("网络错误：无法连接到 %s。检查网络或 URL 是否正确。", host)
		}
		return "网络错误：无法连接到目标主机。检查网络或 URL 是否正确。"
	}

	if strings.Contains(lower, "connection refused") {
		return "目标服务拒绝连接。检查服务是否运行、端口是否正确。"
	}

	if strings.Contains(lower, "no route to host") {
		return "网络不可达：无法路由到目标主机。检查网络连接。"
	}

	// --- Authentication errors ---
	if matchPattern(lower, `unauthorized`, `status 401`, `"status_code":\s*401`) {
		return "认证失败。检查 API Key 是否正确或已过期。"
	}
	if matchPattern(lower, `forbidden`, `status 403`, `"status_code":\s*403`) {
		return "权限不足。检查 API Key 是否有对应权限。"
	}

	// --- Rate limiting ---
	if matchPattern(lower, `rate limit`, `status 429`, `too many requests`) {
		return "请求频率过高，被限流。请稍后重试或降低调用频率。"
	}

	// --- Timeout ---
	if matchPattern(lower, `timeout`, `context deadline exceeded`, `timed out`) {
		return "请求超时。可以在工作流中增加 timeout 参数。"
	}

	// --- File errors ---
	if strings.Contains(lower, "no such file or directory") {
		path := extractFirstGroup(errStr, `(?:stat|open|read|write)\s+"([^"]+)"`)
		if path == "" {
			path = extractFirstGroup(errStr, `([^\s]+\.(?:yaml|yml|json|txt|md|csv|log))`)
		}
		if path != "" {
			return fmt.Sprintf("文件不存在：%s。检查路径是否正确。", path)
		}
		return "文件不存在。检查路径是否正确。"
	}
	if strings.Contains(lower, "permission denied") {
		return "权限不足。检查文件或目录权限。"
	}

	// --- Command execution errors ---
	if matchPattern(lower, `command not found`, `executable file not found`) {
		cmd := extractFirstGroup(errStr, `"([^"]+)"`)
		if cmd == "" {
			cmd = extractFirstGroup(errStr, `command\s+(\S+)`)
		}
		if cmd != "" {
			return fmt.Sprintf("未找到命令：%s。请先安装对应工具。", cmd)
		}
		return "未找到命令。请先安装对应工具。"
	}

	return ""
}

// matchPattern returns true if the lowercased string matches any of the
// given patterns (simple substring or regex).
func matchPattern(s string, patterns ...string) bool {
	for _, p := range patterns {
		// Try regex first for patterns containing regex metacharacters.
		if strings.ContainsAny(p, `.*+?()[]{}|^$\`) {
			if re, err := regexp.Compile(p); err == nil && re.MatchString(s) {
				return true
			}
		}
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

// extractFirstGroup returns the first regex capture group match, or "".
func extractFirstGroup(s, pattern string) string {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return ""
	}
	m := re.FindStringSubmatch(s)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

// troubleshootHint returns a short troubleshooting suggestion based on the
// node type and error, used in the failed-step output (断点13).
func troubleshootHint(nodeName string, err error) string {
	if err == nil {
		return ""
	}
	lower := strings.ToLower(err.Error())
	switch nodeName {
	case "ollama":
		return "排查建议：\n  1. 检查 Ollama 是否运行：ollama list\n  2. 检查模型是否已拉取：ollama pull llama3\n  3. 增加超时时间：在工作流中加 timeout: 60s"
	case "openai", "deepseek", "qwen", "glm", "kimi", "mistral", "baichuan", "internlm", "yi", "xverse", "minimax", "coze", "ima", "mimo":
		if strings.Contains(lower, "api key") || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "401") {
			return "排查建议：\n  1. 检查 API Key 是否正确：aflare config show\n  2. 重新配置：aflare init"
		}
		if strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline") {
			return "排查建议：\n  1. 增加超时时间：在工作流中加 timeout: 60s\n  2. 检查网络连接"
		}
		return "排查建议：\n  1. 检查 API Key：aflare config show\n  2. 检查网络连接\n  3. 检查模型名称是否正确"
	case "http_request", "fetch_url":
		if strings.Contains(lower, "no such host") || strings.Contains(lower, "dial tcp") {
			return "排查建议：\n  1. 检查 URL 是否正确\n  2. 检查网络连接\n  3. 如果是内网地址，检查 VPN 是否连接"
		}
		if strings.Contains(lower, "401") || strings.Contains(lower, "unauthorized") {
			return "排查建议：\n  1. 检查 API Key 或 Token 是否正确\n  2. 检查认证头是否正确设置"
		}
		return "排查建议：\n  1. 检查 URL 是否正确\n  2. 检查网络连接\n  3. 增加重试次数：在工作流中加 retry: 3"
	case "file_read", "file_write":
		return "排查建议：\n  1. 检查文件路径是否正确\n  2. 检查文件权限\n  3. 使用绝对路径避免歧义"
	case "execute":
		if strings.Contains(lower, "metacharacter") {
			return "排查建议：\n  1. 白名单模式下 ; | & $ 引号等 shell 元字符被禁止，参数请用空格直接分隔\n     例如把 echo \"hello\" 改为 echo hello\n  2. 确认命令在白名单中：cat ~/.config/aflare/audit.log 可查看审计记录\n  3. 确需完整 shell 能力时设置 AFLARE_EXECUTE_UNSAFE=1（不安全，自担风险）"
		}
		if strings.Contains(lower, "not found") || strings.Contains(lower, "not in the allowed") {
			return "排查建议：\n  1. 确认命令已安装\n  2. 检查命令是否在 PATH 中\n  3. safe mode 下部分命令被限制，尝试 --safe-mode=L1"
		}
		return "排查建议：\n  1. 检查命令语法是否正确\n  2. 检查命令是否已安装\n  3. 查看完整错误输出"
	default:
		return ""
	}
}

// formatDuration formats a duration as a human-readable string (e.g. "0.3s", "1.2s", "1m30s").
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return d.Round(time.Second).String()
}
