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

	"github.com/charmbracelet/lipgloss"
)

// 渲染相关常量
const (
	maxMarkdownInputBytes = 1 << 20 // 1MB 输入长度上限
	maxMarkdownLineRunes  = 10000   // 单行长度上限（按 rune 计），防超长行 DoS
	defaultMarkdownWidth  = 80      // 默认渲染宽度
	maxMarkdownWidth      = 1000    // 渲染宽度上限（防 DoS：避免 strings.Repeat 分配过大）
)

// 主色与辅助色（与 model.go 保持一致：#7D56F4 紫色主色）
var (
	mdColorPrimary   = lipgloss.Color("#7D56F4")
	mdColorSecondary = lipgloss.Color("#9B7CF6")
	mdColorMuted     = lipgloss.Color("241")
	mdColorCodeFg    = lipgloss.Color("230")
	mdColorCodeBg    = lipgloss.Color("236")
	mdColorInlineBg  = lipgloss.Color("238")
	mdColorLink      = lipgloss.Color("#5B8DEF")
	mdColorQuote     = lipgloss.Color("243")
	mdColorTableSep  = lipgloss.Color("240")
)

// 各级标题样式
var mdHeadingStyles = map[int]lipgloss.Style{
	1: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(mdColorPrimary).Padding(0, 1).MarginBottom(1),
	2: lipgloss.NewStyle().Bold(true).Foreground(mdColorPrimary).MarginBottom(1),
	3: lipgloss.NewStyle().Bold(true).Foreground(mdColorSecondary).MarginBottom(1),
	4: lipgloss.NewStyle().Bold(true).Foreground(mdColorSecondary),
	5: lipgloss.NewStyle().Bold(true).Foreground(mdColorMuted),
	6: lipgloss.NewStyle().Bold(true).Foreground(mdColorMuted),
}

// 代码块样式：带边框
var mdCodeBlockStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(mdColorTableSep).
	Background(mdColorCodeBg).
	Foreground(mdColorCodeFg).
	Padding(0, 1).
	MarginBottom(1)

// 行内代码样式：背景高亮
var mdInlineCodeStyle = lipgloss.NewStyle().
	Background(mdColorInlineBg).
	Foreground(mdColorCodeFg).
	Padding(0, 1)

// 引用块样式：左侧竖线 + 灰色
var mdQuoteStyle = lipgloss.NewStyle().
	BorderLeft(true).
	BorderLeftForeground(mdColorMuted).
	Foreground(mdColorQuote).
	PaddingLeft(1).
	MarginBottom(1)

// 链接样式
var mdLinkStyle = lipgloss.NewStyle().
	Underline(true).
	Foreground(mdColorLink)

// 粗体 / 斜体样式
var mdBoldStyle = lipgloss.NewStyle().Bold(true)
var mdItalicStyle = lipgloss.NewStyle().Italic(true)

// RenderMarkdown 将 Markdown 文本渲染为带样式的终端字符串。
// width 用于换行与表格对齐，<=0 时按 80 处理；输入超 1MB 会被截断。
func RenderMarkdown(md string, width int) string {
	if width <= 0 {
		width = defaultMarkdownWidth
	}
	if width > maxMarkdownWidth {
		width = maxMarkdownWidth
	}

	// 输入长度限制：按字节截断
	if len(md) > maxMarkdownInputBytes {
		md = md[:maxMarkdownInputBytes]
	}

	// 安全：移除 ANSI 转义序列与控制字符，防止终端注入
	md = stripTerminalControl(md)

	lines := strings.Split(md, "\n")
	var out strings.Builder

	inCodeBlock := false
	codeBlockLang := ""
	var codeLines []string

	var quoteLines []string // 引用块缓冲（连续 > 行合并为一块）

	flushQuote := func() {
		if len(quoteLines) == 0 {
			return
		}
		text := strings.Join(quoteLines, "\n")
		out.WriteString(mdQuoteStyle.Render(text))
		out.WriteString("\n")
		quoteLines = quoteLines[:0]
	}

	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		// 单行长度限制：按 rune 截断，防超长行 DoS
		raw = limitLineRunes(raw, maxMarkdownLineRunes)

		// ---- 代码块状态机 ----
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "```") {
			if !inCodeBlock {
				// 进入代码块前先冲掉引用缓冲
				flushQuote()
				inCodeBlock = true
				codeBlockLang = strings.TrimPrefix(trimmed, "```")
				codeLines = codeLines[:0]
				continue
			}
			// 闭合代码块
			inCodeBlock = false
			renderCodeBlock(&out, codeBlockLang, codeLines, width)
			codeBlockLang = ""
			codeLines = codeLines[:0]
			continue
		}
		if inCodeBlock {
			codeLines = append(codeLines, raw)
			continue
		}

		// ---- 引用块 ----
		if strings.HasPrefix(trimmed, ">") {
			content := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
			if content == "" {
				content = ""
			}
			quoteLines = append(quoteLines, content)
			continue
		}
		flushQuote()

		// ---- 空行 ----
		if trimmed == "" {
			out.WriteString("\n")
			continue
		}

		// ---- 水平分隔线 ---/----/**** ----
		if isHorizontalRule(trimmed) {
			out.WriteString(renderHorizontalRule(width))
			out.WriteString("\n")
			continue
		}

		// ---- 标题 ----
		if level, content, ok := parseHeading(raw); ok {
			out.WriteString(mdHeadingStyles[level].Render(content))
			out.WriteString("\n\n")
			continue
		}

		// ---- 表格（行首是 | 且后续还有 | 行才视作表格）----
		if isTableRow(raw) {
			// 收集连续的表格行
			var tableRows []string
			for i < len(lines) && isTableRow(lines[i]) {
				tableRows = append(tableRows, limitLineRunes(lines[i], maxMarkdownLineRunes))
				i++
			}
			i-- // 抵消外层 for 的 i++
			renderTable(&out, tableRows, width)
			out.WriteString("\n")
			continue
		}

		// ---- 列表 ----
		if isListLine(raw) {
			renderListLine(&out, raw)
			continue
		}

		// ---- 普通段落：行内格式 ----
		out.WriteString(renderInline(raw))
		out.WriteString("\n")
	}

	// 收尾：可能未闭合的代码块 / 引用块
	flushQuote()
	if inCodeBlock && len(codeLines) > 0 {
		renderCodeBlock(&out, codeBlockLang, codeLines, width)
	}

	return strings.TrimRight(out.String(), "\n") + "\n"
}

// limitLineRunes 按 rune 数截断单行，避免超长行 DoS
func limitLineRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	rs := []rune(s)
	return string(rs[:max])
}

// isHorizontalRule 判断是否为水平分隔线（--- / *** / ___ 至少 3 个）
func isHorizontalRule(s string) bool {
	if len(s) < 3 {
		return false
	}
	c := s[0]
	if c != '-' && c != '*' && c != '_' {
		return false
	}
	count := 0
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			count++
		} else if s[i] == ' ' || s[i] == '\t' {
			continue
		} else {
			return false
		}
	}
	return count >= 3
}

// renderHorizontalRule 渲染水平分隔线（取终端宽度）
func renderHorizontalRule(width int) string {
	if width <= 0 {
		width = defaultMarkdownWidth
	}
	return lipgloss.NewStyle().Foreground(mdColorMuted).Render(strings.Repeat("─", width))
}

// parseHeading 解析标题行，返回层级与文本内容
func parseHeading(line string) (int, string, bool) {
	s := strings.TrimLeft(line, " ")
	if !strings.HasPrefix(s, "#") {
		return 0, "", false
	}
	level := 0
	for level < len(s) && s[level] == '#' {
		level++
	}
	if level > 6 {
		return 0, "", false
	}
	if level >= len(s) || s[level] != ' ' {
		return 0, "", false
	}
	content := strings.TrimSpace(s[level+1:])
	if content == "" {
		return 0, "", false
	}
	return level, content, true
}

// renderCodeBlock 渲染带边框的代码块
func renderCodeBlock(out *strings.Builder, lang string, lines []string, width int) {
	body := strings.Join(lines, "\n")
	// 语言标识（可选）
	langLabel := strings.TrimSpace(lang)
	if langLabel != "" {
		labelStyle := lipgloss.NewStyle().Foreground(mdColorMuted).Italic(true).MarginBottom(0)
		out.WriteString(labelStyle.Render(langLabel))
		out.WriteString("\n")
	}
	rendered := mdCodeBlockStyle.Render(body)
	out.WriteString(rendered)
	out.WriteString("\n")
}

// isListLine 判断是否为列表行（- / * / + / 数字.）
func isListLine(line string) bool {
	s := strings.TrimLeft(line, " ")
	if s == "" {
		return false
	}
	// 无序列表
	if len(s) >= 2 && (s[0] == '-' || s[0] == '*' || s[0] == '+') && s[1] == ' ' {
		return true
	}
	// 有序列表 数字.
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i > 0 && i < len(s) && s[i] == '.' && i+1 < len(s) && s[i+1] == ' ' {
		return true
	}
	return false
}

// renderListLine 渲染单行列表项
func renderListLine(out *strings.Builder, line string) {
	// 计算缩进
	indent := 0
	for indent < len(line) && line[indent] == ' ' {
		indent++
	}
	content := line[indent:]
	prefix := "  "
	for j := 0; j < indent/2; j++ {
		prefix += "  "
	}

	var bullet string
	var rest string
	if len(content) >= 2 && (content[0] == '-' || content[0] == '*' || content[0] == '+') && content[1] == ' ' {
		bullet = "• "
		rest = content[2:]
	} else {
		// 有序列表
		i := 0
		for i < len(content) && content[i] >= '0' && content[i] <= '9' {
			i++
		}
		num := content[:i]
		bullet = num + ". "
		if i+2 <= len(content) {
			rest = content[i+2:]
		}
	}
	marker := lipgloss.NewStyle().Foreground(mdColorPrimary).Render(bullet)
	out.WriteString(prefix + marker + renderInline(rest))
	out.WriteString("\n")
}

// isTableRow 判断是否为表格行（| 开头并以 | 结尾，或仅含 | 分隔）
func isTableRow(line string) bool {
	s := strings.TrimSpace(line)
	if s == "" {
		return false
	}
	if !strings.HasPrefix(s, "|") {
		return false
	}
	// 至少包含一个分隔单元格
	return strings.Contains(s, "|")
}

// renderTable 渲染简单对齐表格
func renderTable(out *strings.Builder, rows []string, width int) {
	if len(rows) == 0 {
		return
	}
	// 解析所有行
	var parsed [][]string
	maxCols := 0
	for _, row := range rows {
		cells := splitTableRow(row)
		// 跳过分隔行（如 |---|---|）
		if isTableSeparator(cells) {
			continue
		}
		parsed = append(parsed, cells)
		if len(cells) > maxCols {
			maxCols = len(cells)
		}
	}
	if len(parsed) == 0 || maxCols == 0 {
		return
	}

	// 计算每列最大宽度
	colWidths := make([]int, maxCols)
	for _, r := range parsed {
		for i, c := range r {
			w := utf8.RuneCountInString(strings.TrimSpace(c))
			if w > colWidths[i] {
				colWidths[i] = w
			}
		}
	}

	// 限制总宽度，必要时压缩列宽
	total := 0
	for _, w := range colWidths {
		total += w
	}
	// 每列预留 3 个分隔字符（" | "），加上首尾 " "
	overhead := 3*maxCols + 1
	if total+overhead > width && width > overhead {
		avail := width - overhead
		if avail < maxCols {
			avail = maxCols
		}
		// 按比例缩放
		scale := float64(avail) / float64(total)
		for i := range colWidths {
			nw := int(float64(colWidths[i]) * scale)
			if nw < 1 {
				nw = 1
			}
			colWidths[i] = nw
		}
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(mdColorPrimary)
	cellStyle := lipgloss.NewStyle()
	sep := lipgloss.NewStyle().Foreground(mdColorTableSep).Render("|")

	// 渲染表头
	if len(parsed) > 0 {
		out.WriteString(renderTableRow(parsed[0], colWidths, headerStyle, sep))
		out.WriteString("\n")
		// 分隔行
		out.WriteString(renderTableSeparatorRow(colWidths, sep))
		out.WriteString("\n")
	}
	// 渲染数据行
	for i := 1; i < len(parsed); i++ {
		out.WriteString(renderTableRow(parsed[i], colWidths, cellStyle, sep))
		out.WriteString("\n")
	}
}

// splitTableRow 按竖线分割表格行
func splitTableRow(row string) []string {
	s := strings.TrimSpace(row)
	s = strings.TrimPrefix(s, "|")
	s = strings.TrimSuffix(s, "|")
	parts := strings.Split(s, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// isTableSeparator 判断单元格是否为分隔行（--- 等）
func isTableSeparator(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		for i := 0; i < len(c); i++ {
			if c[i] != '-' && c[i] != ':' {
				return false
			}
		}
	}
	return true
}

// renderTableRow 渲染表格一行
func renderTableRow(cells []string, widths []int, style lipgloss.Style, sep string) string {
	var sb strings.Builder
	sb.WriteString(sep)
	for i := 0; i < len(widths); i++ {
		cell := ""
		if i < len(cells) {
			cell = strings.TrimSpace(cells[i])
		}
		// 截断过长单元格
		cell = truncateRunes(cell, widths[i])
		pad := widths[i] - utf8.RuneCountInString(cell)
		if pad < 0 {
			pad = 0
		}
		sb.WriteString(" ")
		sb.WriteString(style.Render(cell))
		if pad > 0 {
			sb.WriteString(strings.Repeat(" ", pad))
		}
		sb.WriteString(" ")
		sb.WriteString(sep)
	}
	return sb.String()
}

// renderTableSeparatorRow 渲染表格分隔行
func renderTableSeparatorRow(widths []int, sep string) string {
	var sb strings.Builder
	sb.WriteString(sep)
	for _, w := range widths {
		sb.WriteString(" ")
		sb.WriteString(strings.Repeat("─", w))
		sb.WriteString(" ")
		sb.WriteString(sep)
	}
	return sb.String()
}

// truncateRunes 按 rune 数截断字符串
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	rs := []rune(s)
	return string(rs[:max])
}

// renderInline 渲染行内格式：粗体、斜体、行内代码、链接
func renderInline(line string) string {
	// 处理顺序：先代码（避免代码内格式被二次处理），再链接，再粗体/斜体
	line = renderInlineCode(line)
	line = renderLinks(line)
	line = renderBold(line)
	line = renderItalic(line)
	return line
}

// renderInlineCode 处理 `code` 形式
func renderInlineCode(line string) string {
	var out strings.Builder
	i := 0
	for i < len(line) {
		if line[i] == '`' {
			// 查找闭合反引号
			end := strings.IndexByte(line[i+1:], '`')
			if end < 0 {
				out.WriteByte(line[i])
				i++
				continue
			}
			code := line[i+1 : i+1+end]
			out.WriteString(mdInlineCodeStyle.Render(code))
			i = i + 1 + end + 1
		} else {
			out.WriteByte(line[i])
			i++
		}
	}
	return out.String()
}

// renderLinks 处理 [text](url) 形式
func renderLinks(line string) string {
	var out strings.Builder
	i := 0
	for i < len(line) {
		if line[i] == '[' {
			// 查找 ]
			closeIdx := strings.IndexByte(line[i+1:], ']')
			if closeIdx < 0 {
				out.WriteByte(line[i])
				i++
				continue
			}
			text := line[i+1 : i+1+closeIdx]
			rest := line[i+1+closeIdx+1:]
			if strings.HasPrefix(rest, "(") {
				closeParen := strings.IndexByte(rest, ')')
				if closeParen < 0 {
					out.WriteByte(line[i])
					i++
					continue
				}
				url := rest[1:closeParen]
				url = sanitizeLabel(url, 200)
				// 渲染：text 带下划线 + url（灰色小字）
				linkText := mdLinkStyle.Render(text)
				urlPart := lipgloss.NewStyle().Foreground(mdColorMuted).Render(" (" + url + ")")
				out.WriteString(linkText + urlPart)
				i = i + 1 + closeIdx + 1 + closeParen + 1
				continue
			}
			// 不是链接形式，原样输出
			out.WriteByte(line[i])
			i++
		} else {
			out.WriteByte(line[i])
			i++
		}
	}
	return out.String()
}

// renderBold 处理 **text** 或 __text__
func renderBold(line string) string {
	return renderEmphasisPair(line, "**", mdBoldStyle)
}

// renderItalic 处理 *text* 或 _text_
func renderItalic(line string) string {
	// 注意：** 会被 renderBold 先消费，这里只处理单个 *
	return renderEmphasisSingle(line, "*", mdItalicStyle)
}

// renderEmphasisPair 处理双字符包裹（如 **text**）
func renderEmphasisPair(line, delim string, style lipgloss.Style) string {
	var out strings.Builder
	i := 0
	for i < len(line) {
		idx := strings.Index(line[i:], delim)
		if idx < 0 {
			out.WriteString(line[i:])
			break
		}
		out.WriteString(line[i : i+idx])
		// 寻找闭合
		rest := line[i+idx+len(delim):]
		closeIdx := strings.Index(rest, delim)
		if closeIdx < 0 {
			out.WriteString(delim)
			out.WriteString(rest)
			break
		}
		content := rest[:closeIdx]
		if content == "" {
			out.WriteString(delim)
			i = i + idx + len(delim)
			continue
		}
		out.WriteString(style.Render(content))
		i = i + idx + len(delim) + closeIdx + len(delim)
	}
	return out.String()
}

// renderEmphasisSingle 处理单字符包裹（如 *text*），需跳过双字符已处理的情况
func renderEmphasisSingle(line, delim string, style lipgloss.Style) string {
	var out strings.Builder
	i := 0
	for i < len(line) {
		// 检查当前位置是否是分隔符
		if i+1 <= len(line) && string(line[i]) == delim {
			// 跳过双字符（已被 renderBold 处理过的会原样保留 delim）
			if i+1 < len(line) && line[i+1] == delim[0] {
				out.WriteByte(line[i])
				out.WriteByte(line[i+1])
				i += 2
				continue
			}
			// 查找闭合的单字符
			rest := line[i+1:]
			closeIdx := -1
			for j := 0; j < len(rest); j++ {
				if rest[j] == delim[0] {
					// 排除 ** 的情况
					if j+1 < len(rest) && rest[j+1] == delim[0] {
						continue
					}
					closeIdx = j
					break
				}
			}
			if closeIdx < 0 {
				out.WriteByte(line[i])
				i++
				continue
			}
			content := rest[:closeIdx]
			if content == "" {
				out.WriteByte(line[i])
				i++
				continue
			}
			out.WriteString(style.Render(content))
			i = i + 1 + closeIdx + 1
			continue
		}
		out.WriteByte(line[i])
		i++
	}
	return out.String()
}

// sanitizeLabel 清洗标签内容：移除控制字符并截断
func sanitizeLabel(s string, max int) string {
	if max <= 0 {
		max = 64
	}
	var sb strings.Builder
	for _, r := range s {
		// 移除控制字符（除空格）
		if r < 0x20 && r != ' ' {
			continue
		}
		if r == 0x7f {
			continue
		}
		sb.WriteRune(r)
	}
	result := sb.String()
	if utf8.RuneCountInString(result) > max {
		rs := []rune(result)
		result = string(rs[:max])
	}
	return result
}

// stripTerminalControl 移除 ANSI 转义序列与控制字符（保留 \n 与 \t），
// 防止用户输入的文本注入终端控制指令（清屏、改色、隐藏光标、退格覆盖等）。
// 供 RenderMarkdown 与 RenderMermaidASCII 在入口处统一调用。
func stripTerminalControl(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		c := s[i]
		if c == 0x1b { // ESC — 可能是 ANSI 转义序列
			if advance := ansiEscSeqLen(s[i:]); advance > 0 {
				i += advance
				continue
			}
			i++ // 无法识别的 ESC，丢弃
			continue
		}
		// 丢弃其他控制字符（保留 \n \t）以及 DEL
		if (c < 0x20 && c != '\n' && c != '\t') || c == 0x7f {
			i++
			continue
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}

// ansiEscSeqLen 返回从 s[0]（应为 ESC）开始的完整 ANSI 转义序列长度。
// 若 s 不是以 ESC 开头的可识别序列，返回 0。
func ansiEscSeqLen(s string) int {
	if len(s) < 2 || s[0] != 0x1b {
		return 0
	}
	switch s[1] {
	case '[':
		// CSI: ESC [ <params> <intermediate> <final 0x40-0x7E>
		i := 2
		for i < len(s) {
			if s[i] >= 0x40 && s[i] <= 0x7e {
				return i + 1
			}
			i++
		}
		return len(s) // 未终止，吃掉剩余
	case ']':
		// OSC: ESC ] <data> (BEL | ST)
		i := 2
		for i < len(s) {
			if s[i] == 0x07 { // BEL
				return i + 1
			}
			if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' { // ST
				return i + 2
			}
			i++
		}
		return len(s)
	case 'P', 'X', '^', '_':
		// DCS / SOS / PM / APC: 终止于 ST (ESC \)
		i := 2
		for i < len(s) {
			if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
				return i + 2
			}
			i++
		}
		return len(s)
	default:
		// 两字符转义序列（如 ESC c 重置终端）
		return 2
	}
}
