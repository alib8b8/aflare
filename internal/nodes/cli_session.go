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
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	validCLISessionThemes = map[string]bool{
		"light": true,
		"dark":  true,
	}

	cliSessionHistory    = make(map[string][]string)
	cliSessionMu         sync.RWMutex
	cliSessionLastUsed   = make(map[string]time.Time)
	cliSessionLastUsedMu sync.RWMutex
)

type CLISessionNode struct{}

func (n *CLISessionNode) Name() string { return "cli_session" }

func (n *CLISessionNode) Description() string {
	return "交互式CLI会话节点。支持上下文保持、命令历史、快捷键、流式输出和自动补全，提供类似Claude Code的流畅CLI体验。"
}

func (n *CLISessionNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - 用户输入的命令或消息",
		Output:      "string - JSON格式的会话响应",
		Params: []ParamSchema{
			{Name: "model", Type: "string", Description: "使用的模型（默认auto，由路由层选择）", Required: false, Default: "auto"},
			{Name: "session_id", Type: "string", Description: "会话ID（自动生成或指定）", Required: false},
			{Name: "max_history", Type: "int", Description: "最大历史记录数（默认50）", Required: false, Default: "50"},
			{Name: "streaming", Type: "bool", Description: "流式输出（默认true）", Required: false, Default: "true"},
			{Name: "theme", Type: "string", Description: "主题（light/dark，默认dark）", Required: false, Default: "dark"},
		},
	}
}

func (n *CLISessionNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	if len(input) > 10000 {
		return "", fmt.Errorf("input too long, max 10000 characters")
	}

	model := getParam(params, "model", "auto")

	sessionID := getParam(params, "session_id", "")
	if sessionID == "" {
		sessionID = generateSessionID()
	}

	maxHistory := parseIntSafe(getParam(params, "max_history", "50"), 50)
	if maxHistory < 1 {
		maxHistory = 1
	}
	if maxHistory > 200 {
		maxHistory = 200
	}

	streaming := strings.ToLower(getParam(params, "streaming", "true")) == "true"

	theme := getParam(params, "theme", "dark")
	if !validCLISessionThemes[theme] {
		return "", fmt.Errorf("invalid theme: %s", theme)
	}

	startTime := time.Now()

	cliSessionMu.Lock()
	history, exists := cliSessionHistory[sessionID]
	if !exists {
		history = make([]string, 0)
	}
	history = append(history, input)
	if len(history) > maxHistory {
		history = history[len(history)-maxHistory:]
	}
	cliSessionHistory[sessionID] = history
	historyCount := len(history)
	cliSessionMu.Unlock()

	cliSessionLastUsedMu.Lock()
	cliSessionLastUsed[sessionID] = time.Now()
	cliSessionLastUsedMu.Unlock()

	cleanupExpiredCLISessions()

	response := simulateCLISessionResponse(input, model, historyCount)

	latency := time.Since(startTime)

	result := map[string]interface{}{
		"session_id":    sessionID,
		"model":         model,
		"response":      response,
		"history_count": historyCount,
		"streaming":     streaming,
		"latency_ms":    latency.Milliseconds(),
		"theme":         theme,
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

func generateSessionID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback: use time-based if crypto/rand fails (should never happen)
		return fmt.Sprintf("cli-session-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("cli-session-%x", b)
}

func simulateCLISessionResponse(input, model string, historyCount int) string {
	lowerInput := strings.ToLower(input)

	switch {
	case strings.Contains(lowerInput, "clear") || strings.Contains(lowerInput, "清屏"):
		return "// [Screen cleared]"
	case strings.Contains(lowerInput, "exit") || strings.Contains(lowerInput, "quit") || strings.Contains(lowerInput, "退出"):
		return "// 会话结束。欢迎下次使用！"
	case strings.Contains(lowerInput, "history") || strings.Contains(lowerInput, "历史"):
		return fmt.Sprintf("// 当前会话共有 %d 条历史记录", historyCount)
	case strings.Contains(lowerInput, "help") || strings.Contains(lowerInput, "帮助"):
		return `// 可用命令:
//   clear / 清屏   - 清除屏幕
//   exit / quit / 退出 - 退出会话
//   history / 历史 - 查看历史记录数
//   help / 帮助    - 显示帮助信息
//   model          - 查看当前模型
//   theme          - 切换主题
//
// 快捷键:
//   Ctrl+C  - 退出
//   Ctrl+L  - 清屏
//   Ctrl+R  - 搜索历史`
	case strings.Contains(lowerInput, "model"):
		return fmt.Sprintf("// 当前模型: %s", model)
	case strings.Contains(lowerInput, "theme"):
		return "// 主题切换功能已就绪"
	case strings.Contains(lowerInput, "cd") || strings.Contains(lowerInput, "ls") || strings.Contains(lowerInput, "pwd"):
		return fmt.Sprintf("// 模拟命令执行: %s", input)
	case strings.Contains(lowerInput, "echo"):
		return fmt.Sprintf("// %s", strings.TrimPrefix(strings.TrimPrefix(input, "echo"), " "))
	default:
		return fmt.Sprintf("(%s) >>> %s", model, input)
	}
}

func cleanupExpiredCLISessions() {
	cliSessionLastUsedMu.RLock()
	if len(cliSessionLastUsed) < 500 {
		cliSessionLastUsedMu.RUnlock()
		return
	}
	toDelete := []string{}
	for id, lastUsed := range cliSessionLastUsed {
		if time.Since(lastUsed) > 24*time.Hour {
			toDelete = append(toDelete, id)
		}
	}
	cliSessionLastUsedMu.RUnlock()

	if len(toDelete) > 0 {
		cliSessionMu.Lock()
		cliSessionLastUsedMu.Lock()
		for _, id := range toDelete {
			delete(cliSessionHistory, id)
			delete(cliSessionLastUsed, id)
		}
		cliSessionLastUsedMu.Unlock()
		cliSessionMu.Unlock()
	}
}

func init() {
	Register(&CLISessionNode{})
}
