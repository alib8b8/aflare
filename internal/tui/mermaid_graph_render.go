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

package tui

import (
	"strings"
	"unicode/utf8"
)

// renderGraphTD 渲染自上而下流程图
func renderGraphTD(nodeMap map[string]*mermaidNode, order []string, edges []*mermaidEdge, width int, ignored []string) string {
	var sb strings.Builder

	if len(order) == 0 {
		sb.WriteString("# 无可渲染节点\n")
		writeIgnored(&sb, ignored)
		return sb.String()
	}

	// 计算每个节点的盒子尺寸
	boxes := make(map[string][]string, len(order))
	maxW := 0
	for _, id := range order {
		node := nodeMap[id]
		box := renderNodeBox(node)
		boxes[id] = box
		if w := boxWidth(box); w > maxW {
			maxW = w
		}
	}
	if maxW > width-4 && width > 4 {
		maxW = width - 4
	}

	// 按出现顺序逐节点绘制，节点之间用 ▼ 连接，边上标签附在箭头旁
	// 出边按 from 分组
	outEdges := make(map[string][]*mermaidEdge)
	for _, e := range edges {
		outEdges[e.from] = append(outEdges[e.from], e)
	}

	// 为简单起见：按 order 顺序绘制节点，节点之后渲染该节点的所有出边箭头
	// 出边只有 1 条时直接画箭头；多条时分叉
	for idx, id := range order {
		if idx > 0 {
			sb.WriteString("\n")
		}
		// 居中节点盒子
		box := boxes[id]
		box = centerBox(box, maxW)
		sb.WriteString(strings.Join(box, "\n"))
		sb.WriteString("\n")

		// 渲染出边
		outs := outEdges[id]
		if len(outs) == 0 {
			continue
		}
		// 中间竖线
		pad := strings.Repeat(" ", maxW/2)
		if len(outs) == 1 {
			e := outs[0]
			if e.label != "" {
				sb.WriteString(pad + "│\n")
				sb.WriteString(pad + "▼ " + e.label + "\n")
			} else {
				sb.WriteString(pad + "│\n")
				sb.WriteString(pad + "▼\n")
			}
		} else {
			// 多条出边：每个目标一行
			sb.WriteString(pad + "│\n")
			for _, e := range outs {
				target := e.to
				if node, ok := nodeMap[target]; ok && node.label != "" {
					target = node.label
				}
				if e.label != "" {
					sb.WriteString(pad + "▼──> " + target + " (" + e.label + ")\n")
				} else {
					sb.WriteString(pad + "▼──> " + target + "\n")
				}
			}
		}
	}

	writeIgnored(&sb, ignored)
	return sb.String()
}

// renderGraphLR 渲染从左到右流程图
func renderGraphLR(nodeMap map[string]*mermaidNode, order []string, edges []*mermaidEdge, width int, ignored []string) string {
	var sb strings.Builder

	if len(order) == 0 {
		sb.WriteString("# 无可渲染节点\n")
		writeIgnored(&sb, ignored)
		return sb.String()
	}

	// 简化版 LR：节点用 [Label] 形式横向串联，以 --> 连接
	// 不做精细的图布局，仅按 order 顺序绘制并标注边
	for i, id := range order {
		node := nodeMap[id]
		label := node.label
		if label == "" {
			label = id
		}
		box := "[" + padLabel(label) + "]"
		sb.WriteString(box)
		// 找到该节点的下一条出边
		var next *mermaidEdge
		for _, e := range edges {
			if e.from == id {
				next = e
				break
			}
		}
		if i < len(order)-1 && next != nil && next.to == order[i+1] {
			if next.label != "" {
				sb.WriteString(" ─" + next.label + "─> ")
			} else {
				sb.WriteString(" ──> ")
			}
		} else if i < len(order)-1 {
			sb.WriteString(" ──> ")
		}
	}
	sb.WriteString("\n")

	writeIgnored(&sb, ignored)
	return sb.String()
}

// renderNodeBox 渲染单个节点为多行 ASCII 框
func renderNodeBox(node *mermaidNode) []string {
	label := node.label
	if label == "" {
		label = node.id
	}
	// 截断过长标签
	label = truncateRunes(label, maxMermaidLabelRunes)
	w := utf8.RuneCountInString(label)
	if w < 4 {
		w = 4
	}
	inner := w + 2

	switch node.shape {
	case "diamond":
		// 菱形
		top := "  " + strings.Repeat("─", inner) + "  "
		mid := "╱" + " " + centerText(label, w) + " " + "╲"
		bot := "  " + strings.Repeat("─", inner) + "  "
		// 顶部和底部尖角
		return []string{
			"     " + strings.Repeat(" ", w/2) + "▲" + strings.Repeat(" ", w/2),
			"    " + top,
			"    " + mid,
			"    " + bot,
			"     " + strings.Repeat(" ", w/2) + "▼" + strings.Repeat(" ", w/2),
		}
	case "circle":
		// 圆形（简化为 ( Label )）
		return []string{
			"  ." + strings.Repeat("-", inner+2) + ".  ",
			" (  " + centerText(label, w) + "  ) ",
			"  '" + strings.Repeat("-", inner+2) + "'  ",
		}
	case "round":
		// 圆角矩形
		top := "╭" + strings.Repeat("─", inner) + "╮"
		mid := "│ " + centerText(label, w) + " │"
		bot := "╰" + strings.Repeat("─", inner) + "╯"
		return []string{top, mid, bot}
	default:
		// rect
		top := "┌" + strings.Repeat("─", inner) + "┐"
		mid := "│ " + centerText(label, w) + " │"
		bot := "└" + strings.Repeat("─", inner) + "┘"
		return []string{top, mid, bot}
	}
}

// centerText 居中文本
func centerText(s string, w int) string {
	sw := utf8.RuneCountInString(s)
	if sw >= w {
		return s
	}
	left := (w - sw) / 2
	right := w - sw - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

// padLabel 给标签两端加空格
func padLabel(s string) string {
	return " " + s + " "
}

// boxWidth 返回盒子最大宽度
func boxWidth(box []string) int {
	max := 0
	for _, l := range box {
		w := utf8.RuneCountInString(l)
		if w > max {
			max = w
		}
	}
	return max
}

// centerBox 将盒子按最大宽度居中（左侧补空格）
func centerBox(box []string, maxW int) []string {
	out := make([]string, len(box))
	for i, l := range box {
		w := utf8.RuneCountInString(l)
		if w >= maxW {
			out[i] = l
			continue
		}
		pad := (maxW - w) / 2
		out[i] = strings.Repeat(" ", pad) + l
	}
	return out
}
