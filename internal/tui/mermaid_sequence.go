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

// renderSequence 渲染时序图（简化为消息列表）
func renderSequence(lines []string, width int) string {
	var sb strings.Builder

	// 解析参与者与消息
	var participants []string
	participantSet := make(map[string]int)
	var msgs []seqMsg

	for _, raw := range lines[1:] {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "%%") || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSuffix(line, ";")

		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "participant ") {
			name := strings.TrimSpace(line[len("participant "):])
			// 去掉 as 别名后面的部分（简化）
			if asIdx := strings.Index(strings.ToLower(name), " as "); asIdx >= 0 {
				name = strings.TrimSpace(name[:asIdx])
			}
			name = sanitizeMermaidLabel(name)
			if name != "" && participantSet[name] == 0 {
				participantSet[name] = len(participants) + 1
				participants = append(participants, name)
			}
			continue
		}

		// Note over A: text
		if strings.HasPrefix(lower, "note over ") {
			rest := strings.TrimSpace(line[len("note over "):])
			colon := strings.Index(rest, ":")
			if colon < 0 {
				continue
			}
			actorPart := strings.TrimSpace(rest[:colon])
			text := strings.TrimSpace(rest[colon+1:])
			msgs = append(msgs, seqMsg{
				noteOver:  true,
				noteActor: sanitizeMermaidLabel(actorPart),
				text:      sanitizeMermaidLabel(text),
			})
			// 注册参与者
			for _, a := range splitActors(actorPart) {
				a = sanitizeMermaidLabel(a)
				if a != "" && participantSet[a] == 0 {
					participantSet[a] = len(participants) + 1
					participants = append(participants, a)
				}
			}
			continue
		}

		// 普通消息 A->>B: text  或  B-->>A: text
		m := parseSeqMessage(line)
		if m != nil {
			msgs = append(msgs, *m)
			// 注册参与者
			for _, a := range []string{m.from, m.to} {
				a = sanitizeMermaidLabel(a)
				if a != "" && participantSet[a] == 0 {
					participantSet[a] = len(participants) + 1
					participants = append(participants, a)
				}
			}
		}
	}

	if len(participants) == 0 {
		sb.WriteString("# 时序图无参与者\n")
		return sb.String()
	}
	if len(participants) > maxMermaidNodes {
		participants = participants[:maxMermaidNodes]
	}

	// 列宽：取参与者名最长，最少 10
	colW := 10
	for _, p := range participants {
		if w := utf8.RuneCountInString(p); w > colW {
			colW = w
		}
	}
	// 限制总宽度
	if len(participants)*colW > width && width > len(participants) {
		colW = width / len(participants)
		if colW < 6 {
			colW = 6
		}
	}

	// 表头：参与者名居中
	for _, p := range participants {
		sb.WriteString(centerText(truncateRunes(p, colW), colW))
		sb.WriteString("   ")
	}
	sb.WriteString("\n")
	// 分隔线
	for range participants {
		sb.WriteString(strings.Repeat("=", colW))
		sb.WriteString("   ")
	}
	sb.WriteString("\n")

	// 消息体
	getCol := func(name string) int {
		// 找到参与者的列索引；找不到时返回 -1
		for i, p := range participants {
			if p == name {
				return i
			}
		}
		return -1
	}

	for _, m := range msgs {
		// 一条空行
		sb.WriteString("\n")
		if m.noteOver {
			// Note over A: text —— 在指定参与者列上方显示备注
			sb.WriteString(renderSeqNote(participants, colW, m.noteActor, m.text))
			continue
		}
		fromCol := getCol(m.from)
		toCol := getCol(m.to)
		if fromCol < 0 || toCol < 0 {
			// 找不到参与者，按文本输出
			sb.WriteString("# " + m.from + " -> " + m.to + ": " + m.text + "\n")
			continue
		}
		sb.WriteString(renderSeqArrow(participants, colW, fromCol, toCol, m.text, m.dashed))
	}

	return sb.String()
}

// parseSeqMessage 解析时序图消息
// 支持 A->>B: msg  /  A-->>B: msg  /  A->B: msg
func parseSeqMessage(line string) *seqMsg {
	// 找冒号分隔消息内容
	colon := strings.Index(line, ":")
	if colon < 0 {
		return nil
	}
	left := strings.TrimSpace(line[:colon])
	text := strings.TrimSpace(line[colon+1:])
	if left == "" {
		return nil
	}

	// 识别箭头
	var from, to string
	var dashed bool
	switch {
	case strings.Contains(left, "-->>"):
		parts := strings.SplitN(left, "-->>", 2)
		from = strings.TrimSpace(parts[0])
		to = strings.TrimSpace(parts[1])
		dashed = true
	case strings.Contains(left, "->>"):
		parts := strings.SplitN(left, "->>", 2)
		from = strings.TrimSpace(parts[0])
		to = strings.TrimSpace(parts[1])
	case strings.Contains(left, "-->"):
		parts := strings.SplitN(left, "-->", 2)
		from = strings.TrimSpace(parts[0])
		to = strings.TrimSpace(parts[1])
		dashed = true
	case strings.Contains(left, "->"):
		parts := strings.SplitN(left, "->", 2)
		from = strings.TrimSpace(parts[0])
		to = strings.TrimSpace(parts[1])
	default:
		return nil
	}
	if from == "" || to == "" {
		return nil
	}
	return &seqMsg{
		from:   sanitizeMermaidLabel(from),
		to:     sanitizeMermaidLabel(to),
		text:   sanitizeMermaidLabel(text),
		dashed: dashed,
	}
}

// splitActors 拆分 Note over 中的参与者（支持 "A,B" 或 "A,B,C"）
func splitActors(s string) []string {
	s = strings.TrimSpace(s)
	// 去掉可能的逗号
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// renderSeqArrow 渲染时序图的一条箭头
func renderSeqArrow(participants []string, colW, fromCol, toCol int, text string, dashed bool) string {
	var sb strings.Builder
	// 每列画生命线（│），from-to 之间画箭头
	// 一行：每个参与者占 colW+3 字符
	gap := 3
	totalCols := len(participants)

	// 生命线行
	for i := 0; i < totalCols; i++ {
		if i == fromCol || i == toCol {
			sb.WriteString(strings.Repeat(" ", colW/2) + "│" + strings.Repeat(" ", colW-colW/2-1))
		} else {
			sb.WriteString(strings.Repeat(" ", colW))
		}
		sb.WriteString(strings.Repeat(" ", gap))
	}
	sb.WriteString("\n")

	// 箭头行：from 列发出，to 列接收
	// 先构造一个 char 数组
	lineWidth := totalCols * (colW + gap)
	line := make([]byte, lineWidth)
	for i := range line {
		line[i] = ' '
	}

	// 起点和终点位置（每列中央）
	fromX := fromCol*(colW+gap) + colW/2
	toX := toCol*(colW+gap) + colW/2
	if fromX > toX {
		fromX, toX = toX, fromX
	}
	// 画箭头线
	arrow := byte('-')
	if dashed {
		arrow = byte('.')
	}
	for x := fromX + 1; x < toX; x++ {
		line[x] = arrow
	}
	// 箭头头
	if toCol >= fromCol {
		line[toX-1] = '>'
	} else {
		// 反向：原本 fromCol>toCol 已交换，这里通过参数判断方向
		line[fromX+1] = '<'
		// 等价：箭头头在原 from 侧
	}

	// 在箭头中央放文本
	if text != "" {
		text = truncateRunes(text, toX-fromX-2)
		tw := utf8.RuneCountInString(text)
		if tw > 0 {
			start := fromX + 1 + ((toX-fromX-1)-tw)/2
			if start < fromX+1 {
				start = fromX + 1
			}
			rs := []rune(text)
			for k, r := range rs {
				pos := start + k
				if pos >= 0 && pos < lineWidth {
					// 写入 rune 的字节
					bs := string(r)
					for bi := 0; bi < len(bs); bi++ {
						if pos+bi < lineWidth {
							line[pos+bi] = bs[bi]
						}
					}
				}
			}
		}
	}

	sb.WriteString(string(line))
	sb.WriteString("\n")
	return sb.String()
}

// renderSeqNote 渲染 Note over 备注
func renderSeqNote(participants []string, colW int, actor, text string) string {
	var sb strings.Builder
	// 找到 actor 所在列（支持 "A,B" 形式覆盖范围）
	actors := splitActors(actor)
	startCol, endCol := -1, -1
	for i, p := range participants {
		for _, a := range actors {
			if p == a {
				if startCol < 0 {
					startCol = i
				}
				endCol = i
			}
		}
	}
	if startCol < 0 {
		// 找不到，按文本输出
		sb.WriteString("# Note over " + actor + ": " + text + "\n")
		return sb.String()
	}
	gap := 3
	totalCols := len(participants)
	lineWidth := totalCols * (colW + gap)
	startX := startCol * (colW + gap)
	endX := (endCol+1)*(colW+gap) - gap
	noteW := endX - startX
	if noteW < 4 {
		noteW = 4
	}

	// 备注文本截断
	label := truncateRunes(text, noteW-4)
	lw := utf8.RuneCountInString(label)

	// 顶边
	top := make([]byte, lineWidth)
	for i := range top {
		top[i] = ' '
	}
	for x := startX; x < endX; x++ {
		if x >= 0 && x < lineWidth {
			top[x] = '-'
		}
	}
	// 中间行：| label |
	mid := make([]byte, lineWidth)
	for i := range mid {
		mid[i] = ' '
	}
	if startX >= 0 && startX < lineWidth {
		mid[startX] = '|'
	}
	if endX-1 >= 0 && endX-1 < lineWidth {
		mid[endX-1] = '|'
	}
	// 居中放置文本
	if lw > 0 {
		textStart := startX + 1 + (noteW-2-lw)/2
		if textStart < startX+1 {
			textStart = startX + 1
		}
		rs := []rune(label)
		for k, r := range rs {
			bs := string(r)
			for bi := 0; bi < len(bs); bi++ {
				pos := textStart + k + bi
				if pos >= 0 && pos < lineWidth && pos < endX-1 {
					mid[pos] = bs[bi]
				}
			}
		}
	}
	// 底边
	bot := make([]byte, lineWidth)
	copy(bot, top)

	sb.WriteString(string(top))
	sb.WriteString("\n")
	sb.WriteString(string(mid))
	sb.WriteString("\n")
	sb.WriteString(string(bot))
	sb.WriteString("\n")
	return sb.String()
}
