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

package nodes

import (
	"path/filepath"
	"regexp"
	"strings"
)

// ============================================================================
// 工具可移植性兼容层（tools_compat.go）
//
// 借鉴 SpaceX 开源 Grok Build 从 Codex/OpenCode 移植工具的做法，为
// llm-box 工作流引擎增加主流 AI Agent 工具接口兼容层，使从
// Codex/OpenCode/Grok Build 迁移的工作流可以直接使用熟悉的工具名。
//
// 实现的兼容节点：
//   - glob        （借鉴 Codex / OpenCode glob）        → tools_compat_glob.go
//   - grep        （借鉴 Codex grep_files / OpenCode grep） → tools_compat_grep.go
//   - list_dir    （借鉴 Codex list_dir）                → tools_compat_listdir.go
//   - apply_patch （借鉴 Codex apply_patch）             → tools_compat_applypatch.go
//
// llm-box 是独立实现，仅借鉴工具名和概念，不复制源码。
// 详见同目录 THIRD_PARTY_NOTICES.md。
//
// 所有新增类型/函数均带 tc 前缀，避免与包内已有符号冲突。
// ============================================================================

// 通用安全限制常量
const (
	tcMaxGlobPatternLen  = 256              // glob 模式最大长度
	tcMaxGrepPatternLen  = 1024             // grep 正则最大长度
	tcMaxWalkDepth       = 10               // glob/grep 递归深度上限
	tcMaxListDirDepth    = 5                // list_dir 递归深度上限
	tcMaxGlobResults     = 10000            // glob 结果数上限
	tcDefaultMaxGrep     = 1000             // grep 默认最大匹配数
	tcMaxFileScanSize    = 10 * 1024 * 1024 // grep 单文件大小上限 10MB
	tcDefaultListEntries = 1000             // list_dir 默认最大条目数
	tcMaxPatchSize       = 1024 * 1024      // apply_patch 补丁大小上限 1MB
	tcMaxPatchFiles      = 50               // apply_patch 最多修改文件数
	tcHardMaxGrep        = 100000           // grep 最大匹配数硬上限（防 DoS）
	tcHardMaxListEntries = 100000           // list_dir 最大条目数硬上限（防 DoS）
)

// 预编译正则（包级变量）。
// 注意：grep 的用户 pattern 仍需运行时编译并处理错误。
var (
	// apply_patch 的 hunk 头：@@ -oldStart,oldCount +newStart,newCount @@ 可选上下文
	tcHunkHeaderRe = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)
	// 控制字符清洗（保留 \t \n \r）
	tcControlCharRe = regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]`)
)

// tcSanitize 清洗输出中的控制字符（保留 \t \n \r），防止终端注入
func tcSanitize(s string) string {
	return tcControlCharRe.ReplaceAllString(s, "?")
}

// tcSanitizePath 清洗路径输出：将所有控制字符（含 \t \n \r）替换为 '?'。
// 用于行式输出中的文件路径，防止文件名含换行符破坏输出格式或注入。
func tcSanitizePath(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= 0x20 && r != 0x7f {
			b.WriteRune(r)
		} else {
			b.WriteByte('?')
		}
	}
	return b.String()
}

// tcRelDepth 返回相对路径的目录深度（用于递归深度限制）
func tcRelDepth(rel string) int {
	rel = filepath.ToSlash(rel)
	if rel == "." || rel == "" {
		return 0
	}
	return strings.Count(rel, "/") + 1
}
