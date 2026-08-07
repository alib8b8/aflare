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

package tui

import (
	"strings"
)

// renderGraph 渲染流程图（graph TD / graph LR）
func renderGraph(lines []string, width int) string {
	// 解析方向（默认 TD）
	direction := "TD"
	first := strings.TrimSpace(lines[0])
	fields := strings.Fields(first)
	if len(fields) >= 2 {
		direction = strings.ToUpper(fields[1])
	}
	if direction != "TD" && direction != "LR" {
		direction = "TD"
	}

	nodeMap := make(map[string]*mermaidNode)
	var nodeOrder []string
	var edges []*mermaidEdge
	var ignored []string

	for _, raw := range lines[1:] {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "%%") || strings.HasPrefix(line, "#") {
			continue
		}
		// 移除末尾分号
		line = strings.TrimSuffix(line, ";")
		if line == "" {
			continue
		}

		// 尝试解析为边定义（含节点定义）
		edge := parseGraphEdge(line)
		if edge != nil {
			// 注册节点（若边定义里携带了节点形状/标签）
			registerNodeFromText(nodeMap, &nodeOrder, edge.fromRaw)
			registerNodeFromText(nodeMap, &nodeOrder, edge.toRaw)
			if len(edges) < maxMermaidEdges {
				edges = append(edges, edge)
			} else {
				ignored = append(ignored, "# 边数已达上限("+itoa(maxMermaidEdges)+")，忽略: "+line)
			}
			continue
		}

		// 尝试解析为纯节点定义 A[Label]
		if def := parseStandaloneNode(line); def != "" {
			registerNodeFromText(nodeMap, &nodeOrder, line)
			continue
		}

		ignored = append(ignored, "# 无法解析: "+line)
	}

	// 节点数限制
	if len(nodeOrder) > maxMermaidNodes {
		ignored = append(ignored, "# 节点数已达上限("+itoa(maxMermaidNodes)+")，仅渲染前 "+itoa(maxMermaidNodes)+" 个")
		nodeOrder = nodeOrder[:maxMermaidNodes]
	}

	if direction == "LR" {
		return renderGraphLR(nodeMap, nodeOrder, edges, width, ignored)
	}
	return renderGraphTD(nodeMap, nodeOrder, edges, width, ignored)
}

// parseGraphEdge 解析一条边定义，返回 nil 表示不是边
// 支持：A --> B / A -->|text| B / A --- B / A -.-> B / A --> B[Label] 等
func parseGraphEdge(line string) *mermaidEdge {
	// 边分隔符及其样式（顺序很重要：长分隔符优先匹配）
	type sepMatch struct {
		sep   string
		style string
	}

	seps := []sepMatch{
		{"-.->", "dotted"},
		{"-->", "solid"},
		{"---", "solid"},
		{"->", "solid"},
		{"-", "solid"},
	}

	for _, sm := range seps {
		if idx := strings.Index(line, sm.sep); idx >= 0 {
			left := strings.TrimSpace(line[:idx])
			rest := strings.TrimSpace(line[idx+len(sm.sep):])
			if left == "" || rest == "" {
				continue
			}
			// 处理 -->|label| 形式
			label := ""
			if strings.HasPrefix(rest, "|") {
				endPipe := strings.Index(rest[1:], "|")
				if endPipe >= 0 {
					label = strings.TrimSpace(rest[1 : 1+endPipe])
					rest = strings.TrimSpace(rest[2+endPipe:])
				}
			}
			if rest == "" {
				continue
			}
			// 处理右侧带标签的情况（如 --> B[label]）
			right := rest
			// 拆分可能的空格隔断（如 B[Label] C[Label2]）
			if spIdx := firstSpaceBeforeBracket(rest); spIdx > 0 {
				right = strings.TrimSpace(rest[:spIdx])
			}
			return &mermaidEdge{
				from:    extractNodeID(left),
				to:      extractNodeID(right),
				fromRaw: left,
				toRaw:   right,
				label:   sanitizeMermaidLabel(label),
				style:   sm.style,
			}
		}
	}
	return nil
}

// firstSpaceBeforeBracket 找到第一个不在括号内的空格位置，用于拆分多个节点
func firstSpaceBeforeBracket(s string) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '[', '(', '{':
			depth++
		case ']', ')', '}':
			if depth > 0 {
				depth--
			}
		case ' ':
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// parseStandaloneNode 判断是否为纯节点定义（如 A[Label]），返回节点 ID
func parseStandaloneNode(line string) string {
	id := extractNodeID(line)
	if id == "" {
		return ""
	}
	// 必须包含括号才认为是节点定义
	if !strings.ContainsAny(line, "[({") {
		return ""
	}
	return id
}

// registerNodeFromText 从一段文本中识别并注册节点
func registerNodeFromText(nodeMap map[string]*mermaidNode, order *[]string, text string) {
	id, label, shape := parseNodeSpec(text)
	if id == "" {
		return
	}
	if _, exists := nodeMap[id]; !exists {
		node := &mermaidNode{id: id, label: label, shape: shape}
		// 若无显式标签，则用 id 兜底
		if node.label == "" {
			node.label = id
		}
		nodeMap[id] = node
		*order = append(*order, id)
	} else if label != "" {
		// 已存在节点，但本次提供了标签 → 更新
		nodeMap[id].label = label
		nodeMap[id].shape = shape
	}
}

// parseNodeSpec 解析节点定义，返回 id、label、shape
// 支持：A / A[Label] / A(Label) / A{Label} / A((Label))
func parseNodeSpec(text string) (id, label, shape string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", "", ""
	}
	// 找到第一个非标识符字符
	i := 0
	for i < len(text) {
		c := text[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			i++
			continue
		}
		break
	}
	id = text[:i]
	if id == "" {
		return "", "", ""
	}
	rest := strings.TrimSpace(text[i:])
	if rest == "" {
		return id, "", ""
	}

	// 根据下一个字符判断形状
	switch rest[0] {
	case '[':
		// A[Label]  或  A[[Label]]
		inner, ok := extractBracket(rest, '[', ']')
		if !ok {
			return id, "", ""
		}
		// 处理 [[Label]] 双层
		if strings.HasPrefix(inner, "[") && strings.HasSuffix(inner, "]") {
			inner = inner[1 : len(inner)-1]
		}
		return id, sanitizeMermaidLabel(inner), "rect"
	case '(':
		// A(Label)  或  A((Label))
		if strings.HasPrefix(rest, "((") {
			inner, ok := extractBracket(rest, '(', ')')
			if !ok {
				return id, "", ""
			}
			// 去掉双括号
			if strings.HasPrefix(inner, "(") && strings.HasSuffix(inner, ")") {
				inner = inner[1 : len(inner)-1]
			}
			return id, sanitizeMermaidLabel(inner), "circle"
		}
		inner, ok := extractBracket(rest, '(', ')')
		if !ok {
			return id, "", ""
		}
		return id, sanitizeMermaidLabel(inner), "round"
	case '{':
		// A{Label}
		inner, ok := extractBracket(rest, '{', '}')
		if !ok {
			return id, "", ""
		}
		return id, sanitizeMermaidLabel(inner), "diamond"
	default:
		return id, "", ""
	}
}

// extractBracket 提取首尾括号之间的内容（不处理嵌套，仅去最外层）
func extractBracket(s string, open, close byte) (string, bool) {
	if len(s) < 2 || s[0] != open {
		return "", false
	}
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return s[1:i], true
			}
		}
	}
	return "", false
}

// extractNodeID 从文本中提取节点 ID（仅字母数字下划线短横）
func extractNodeID(text string) string {
	text = strings.TrimSpace(text)
	i := 0
	for i < len(text) {
		c := text[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			i++
			continue
		}
		break
	}
	return text[:i]
}
