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
	"unicode/utf8"
)

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
