package tui

import (
	"strings"
	"unicode/utf8"
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

// sanitizeMermaidLabel 清洗 Mermaid 标签：移除控制字符、双引号、反引号，并截断
func sanitizeMermaidLabel(s string) string {
	s = strings.TrimSpace(s)
	// 去掉首尾成对的双引号
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	var sb strings.Builder
	for _, r := range s {
		// 移除控制字符
		if r < 0x20 && r != ' ' {
			continue
		}
		if r == 0x7f {
			continue
		}
		// 移除可能干扰 ASCII 布局的字符
		if r == '\n' || r == '\r' || r == '\t' {
			sb.WriteRune(' ')
			continue
		}
		sb.WriteRune(r)
	}
	result := strings.TrimSpace(sb.String())
	if utf8.RuneCountInString(result) > maxMermaidLabelRunes {
		rs := []rune(result)
		result = string(rs[:maxMermaidLabelRunes])
	}
	return result
}

// writeIgnored 写入被忽略的语法提示
func writeIgnored(sb *strings.Builder, ignored []string) {
	if len(ignored) == 0 {
		return
	}
	sb.WriteString("\n")
	for _, line := range ignored {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
}

// itoa 简易整数转字符串（避免引入 strconv 仅为这一个用途）
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
