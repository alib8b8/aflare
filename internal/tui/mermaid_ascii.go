// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌‌‌​‌​‌‌​​​​‌​‌‌​‌​​‌​​‌​​‌​‌​‌​​‌​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​‌​​​​​‌​‌​​‌​​‌⁠
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

package tui

import (
	"strings"
)

// Mermaid 渲染相关常量
const (
	maxMermaidInputBytes = 256 << 10 // 256KB 输入长度上限
	maxMermaidNodes      = 50        // 节点数上限（防 DoS）
	maxMermaidEdges      = 100       // 边数上限（防 DoS）
	maxMermaidLabelRunes = 64        // 节点/边标签长度上限
	defaultMermaidWidth  = 80        // 默认渲染宽度
	maxMermaidWidth      = 1000      // 渲染宽度上限（防 DoS）
)

// mermaidNode 表示一个流程图节点
type mermaidNode struct {
	id    string
	label string
	shape string // rect / round / diamond / circle
}

// mermaidEdge 表示一条流程图边
type mermaidEdge struct {
	from    string
	to      string
	fromRaw string // 原始左侧文本（含节点定义）
	toRaw   string // 原始右侧文本（含节点定义）
	label   string
	style   string // solid / dotted
}

// seqMsg 表示时序图中的一条消息（普通箭头或 Note over）
type seqMsg struct {
	from      string
	to        string
	text      string
	dashed    bool // reply 用虚线
	noteOver  bool
	noteActor string
}

// RenderMermaidASCII 将 Mermaid 图表文本转为 ASCII art。
// 支持 graph TD/LR 流程图与 sequenceDiagram 时序图。
// width 用于控制换行，<=0 时按 80 处理；输入超 256KB 会被截断。
// 不支持的语法会被忽略并以注释形式提示。
func RenderMermaidASCII(mermaid string, width int) string {
	if width <= 0 {
		width = defaultMermaidWidth
	}
	if width > maxMermaidWidth {
		width = maxMermaidWidth
	}
	// 输入长度限制
	if len(mermaid) > maxMermaidInputBytes {
		mermaid = mermaid[:maxMermaidInputBytes]
	}
	// 安全：移除 ANSI 转义序列与控制字符，防止终端注入
	mermaid = stripTerminalControl(mermaid)

	lines := strings.Split(mermaid, "\n")
	if len(lines) == 0 {
		return ""
	}

	// 跳过可能的 ```mermaid 围栏
	startIdx := 0
	if strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
		startIdx = 1
	}
	// 去掉末尾的 ```
	endIdx := len(lines)
	if endIdx > startIdx && strings.HasPrefix(strings.TrimSpace(lines[endIdx-1]), "```") {
		endIdx--
	}
	contentLines := lines[startIdx:endIdx]
	if len(contentLines) == 0 {
		return ""
	}

	// 识别图表类型
	first := strings.TrimSpace(contentLines[0])
	lower := strings.ToLower(first)

	switch {
	case strings.HasPrefix(lower, "graph") || strings.HasPrefix(lower, "flowchart"):
		return renderGraph(contentLines, width)
	case strings.HasPrefix(lower, "sequencediagram"):
		return renderSequence(contentLines, width)
	default:
		var sb strings.Builder
		sb.WriteString("# 不支持的 Mermaid 类型，仅支持 graph/flowchart 与 sequenceDiagram\n")
		// 原样回显（已做长度限制）
		for _, l := range contentLines {
			sb.WriteString("# ")
			sb.WriteString(l)
			sb.WriteString("\n")
		}
		return sb.String()
	}
}
