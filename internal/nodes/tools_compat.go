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
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/alib8b8/llm-box/internal/logger"
)

// ============================================================================
// 工具可移植性兼容层（tools_compat.go）
//
// 借鉴 SpaceX 开源 Grok Build 从 Codex/OpenCode 移植工具的做法，为
// llm-box 工作流引擎增加主流 AI Agent 工具接口兼容层，使从
// Codex/OpenCode/Grok Build 迁移的工作流可以直接使用熟悉的工具名。
//
// 实现的兼容节点：
//   - glob        （借鉴 Codex / OpenCode glob）
//   - grep        （借鉴 Codex grep_files / OpenCode grep）
//   - list_dir    （借鉴 Codex list_dir）
//   - apply_patch （借鉴 Codex apply_patch）
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

// ----------------------------------------------------------------------------
// glob 工具：文件通配匹配（借鉴 Codex / OpenCode glob）
// ----------------------------------------------------------------------------

// TCGlobNode 文件通配匹配节点
type TCGlobNode struct{}

func init() {
	Register(&TCGlobNode{})
}

// Name 返回节点名
func (n *TCGlobNode) Name() string { return "glob" }

// Description 返回节点描述
func (n *TCGlobNode) Description() string {
	return "Glob-match files (compat: Codex/OpenCode glob)"
}

// Schema 返回节点 schema
func (n *TCGlobNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "glob",
		Description: "递归匹配文件路径，返回匹配的文件列表（每行一个）。兼容 Codex/OpenCode 的 glob 工具。",
		Input:       "string - 未使用",
		Output:      "string - 匹配的文件路径列表（每行一个，相对搜索根目录）",
		Params: []ParamSchema{
			{Name: "pattern", Type: "string", Description: "glob 模式，如 **/*.go 或 *.md", Required: true},
			{Name: "path", Type: "string", Description: "搜索根目录（默认工作目录）", Required: false, Default: "."},
		},
	}
}

// Execute 实现 Node 接口
func (n *TCGlobNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	pattern := params["pattern"]
	if pattern == "" {
		return "", fmt.Errorf("pattern parameter is required")
	}
	if len(pattern) > tcMaxGlobPatternLen {
		return "", fmt.Errorf("pattern too long (max %d chars)", tcMaxGlobPatternLen)
	}
	// 拒绝含 .. 的 pattern（防止路径遍历）
	if strings.Contains(pattern, "..") {
		return "", fmt.Errorf("pattern must not contain '..'")
	}

	root := getParam(params, "path", ".")
	safeRoot, err := validateReadPath(root)
	if err != nil {
		return "", fmt.Errorf("path validation failed: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}

	var results []string
	walkErr := filepath.WalkDir(safeRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // 跳过无法访问的条目
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		// 跳过符号链接（WalkDir 默认不跟踪，但符号链接条目仍会出现在列表中）
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel, rerr := filepath.Rel(safeRoot, path)
		if rerr != nil {
			return nil
		}
		// 递归深度限制
		if d.IsDir() {
			if tcRelDepth(rel) > tcMaxWalkDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if tcRelDepth(rel) > tcMaxWalkDepth {
			return nil
		}
		if tcMatchGlob(pattern, rel) {
			results = append(results, tcSanitizePath(filepath.ToSlash(rel)))
			if len(results) >= tcMaxGlobResults {
				return filepath.SkipAll
			}
		}
		return nil
	})
	if walkErr != nil {
		return "", fmt.Errorf("walk failed: %w", walkErr)
	}

	logger.Info("glob matched files", "pattern", pattern, "count", len(results))
	return strings.Join(results, "\n"), nil
}

// tcMatchGlob 用 filepath.Match 实现 glob 匹配，并补充支持 ** 通配
// （filepath.Match 原生不支持 **，这里手工处理前缀/后缀）
func tcMatchGlob(pattern, relPath string) bool {
	relPath = filepath.ToSlash(relPath)
	pattern = filepath.ToSlash(pattern)

	// 支持 ** 通配（如 src/**/*.go 或 **/*.go）
	if strings.Contains(pattern, "**") {
		idx := strings.Index(pattern, "**")
		prefix := pattern[:idx]
		suffix := strings.TrimPrefix(pattern[idx+2:], "/")

		// 校验前缀
		if prefix != "" {
			prefix = strings.TrimSuffix(prefix, "/")
			if relPath != prefix && !strings.HasPrefix(relPath, prefix+"/") {
				return false
			}
		}
		if suffix == "" {
			return true
		}
		// suffix 用 filepath.Match 匹配 basename
		if matched, _ := filepath.Match(suffix, filepath.Base(relPath)); matched {
			return true
		}
		return false
	}

	// 无 ** 模式：先匹配相对路径，再匹配 basename
	if matched, _ := filepath.Match(pattern, relPath); matched {
		return true
	}
	if matched, _ := filepath.Match(pattern, filepath.Base(relPath)); matched {
		return true
	}
	return false
}

// ----------------------------------------------------------------------------
// grep 工具：文件内容搜索（借鉴 Codex grep_files / OpenCode grep）
// ----------------------------------------------------------------------------

// TCGrepNode 文件内容搜索节点
type TCGrepNode struct{}

func init() {
	Register(&TCGrepNode{})
}

// Name 返回节点名
func (n *TCGrepNode) Name() string { return "grep" }

// Description 返回节点描述
func (n *TCGrepNode) Description() string {
	return "Search file contents by regex (compat: Codex grep_files / OpenCode grep)"
}

// Schema 返回节点 schema
func (n *TCGrepNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "grep",
		Description: "递归搜索文件内容，返回匹配行（格式：file:line:content）。兼容 Codex/OpenCode 的 grep 工具。",
		Input:       "string - 未使用",
		Output:      "string - 匹配行列表（每行一条：file:line:content）",
		Params: []ParamSchema{
			{Name: "pattern", Type: "string", Description: "正则表达式", Required: true},
			{Name: "path", Type: "string", Description: "搜索目录（默认工作目录）", Required: false, Default: "."},
			{Name: "glob", Type: "string", Description: "文件名过滤，如 *.go", Required: false},
			{Name: "ignore_case", Type: "string", Description: "是否忽略大小写：true/false（默认 false）", Required: false, Default: "false"},
			{Name: "max_matches", Type: "string", Description: "最大匹配数（默认 1000）", Required: false, Default: "1000"},
		},
	}
}

// Execute 实现 Node 接口
func (n *TCGrepNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	pattern := params["pattern"]
	if pattern == "" {
		return "", fmt.Errorf("pattern parameter is required")
	}
	if len(pattern) > tcMaxGrepPatternLen {
		return "", fmt.Errorf("pattern too long (max %d chars)", tcMaxGrepPatternLen)
	}

	// ignore_case 处理：用 (?i) 内联标志
	ignoreCase := getParam(params, "ignore_case", "false")
	if ignoreCase == "true" || ignoreCase == "1" {
		pattern = "(?i)" + pattern
	}
	re, err := compileRegexCached(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex: %w", err)
	}

	// 可选 glob 文件名过滤
	globFilter := params["glob"]
	if globFilter != "" {
		if strings.Contains(globFilter, "..") {
			return "", fmt.Errorf("glob filter must not contain '..'")
		}
	}

	// max_matches
	maxMatches := tcDefaultMaxGrep
	if v := params["max_matches"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxMatches = n
		}
	}
	if maxMatches > tcHardMaxGrep {
		maxMatches = tcHardMaxGrep
	}

	root := getParam(params, "path", ".")
	safeRoot, err := validateReadPath(root)
	if err != nil {
		return "", fmt.Errorf("path validation failed: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}

	var matches []string
	walkErr := filepath.WalkDir(safeRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		// 跳过符号链接
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel, rerr := filepath.Rel(safeRoot, path)
		if rerr != nil {
			return nil
		}
		if d.IsDir() {
			if tcRelDepth(rel) > tcMaxWalkDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if tcRelDepth(rel) > tcMaxWalkDepth {
			return nil
		}
		// 应用 glob 文件名过滤
		if globFilter != "" {
			matched, _ := filepath.Match(globFilter, filepath.Base(rel))
			if !matched {
				return nil
			}
		}
		// 应用 max_matches 提前终止
		if len(matches) >= maxMatches {
			return filepath.SkipAll
		}
		// 单文件搜索
		fileMatches, err := tcGrepFile(path, rel, re, maxMatches-len(matches))
		if err != nil {
			// 跳过无法读取/过大/二进制的文件，不中断整个搜索
			return nil
		}
		matches = append(matches, fileMatches...)
		return nil
	})
	if walkErr != nil {
		return "", fmt.Errorf("walk failed: %w", walkErr)
	}

	logger.Info("grep matched lines", "pattern", pattern, "count", len(matches))
	return strings.Join(matches, "\n"), nil
}

// tcGrepFile 在单个文件中搜索匹配行
func tcGrepFile(absPath, relPath string, re *regexp.Regexp, limit int) ([]string, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	if info.Size() > tcMaxFileScanSize {
		return nil, fmt.Errorf("file too large: %s (%d bytes)", relPath, info.Size())
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	// 跳过二进制文件（检测 \x00）
	if bytesIndexByte(data, 0) >= 0 {
		return nil, fmt.Errorf("binary file skipped: %s", relPath)
	}

	var matches []string
	lineNum := 0
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	// 提高单行容量上限（默认 64KB），避免长行扫描失败
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if re.MatchString(line) {
			// 输出格式：file:line:content，路径与内容均清洗控制字符
			matches = append(matches, fmt.Sprintf("%s:%d:%s", tcSanitizePath(relPath), lineNum, tcSanitize(line)))
			if len(matches) >= limit {
				break
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return matches, nil
}

// bytesIndexByte 返回字节 b 在 s 中第一次出现的索引，未找到返回 -1
// （独立实现避免引入 bytes 包仅为这一个调用）
func bytesIndexByte(s []byte, b byte) int {
	for i, c := range s {
		if c == b {
			return i
		}
	}
	return -1
}

// ----------------------------------------------------------------------------
// list_dir 工具：列出目录内容（借鉴 Codex list_dir）
// ----------------------------------------------------------------------------

// TCListDirNode 列出目录内容节点
type TCListDirNode struct{}

func init() {
	Register(&TCListDirNode{})
}

// Name 返回节点名
func (n *TCListDirNode) Name() string { return "list_dir" }

// Description 返回节点描述
func (n *TCListDirNode) Description() string {
	return "List directory entries (compat: Codex list_dir)"
}

// Schema 返回节点 schema
func (n *TCListDirNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "list_dir",
		Description: "列出目录内容，返回 name/type/size 列表。兼容 Codex 的 list_dir 工具。",
		Input:       "string - 未使用",
		Output:      "string - 目录条目列表（每行一条：relpath\ttype\tsize）",
		Params: []ParamSchema{
			{Name: "path", Type: "string", Description: "目录路径", Required: true},
			{Name: "recursive", Type: "string", Description: "是否递归：true/false（默认 false）", Required: false, Default: "false"},
			{Name: "max_entries", Type: "string", Description: "最大条目数（默认 1000）", Required: false, Default: "1000"},
		},
	}
}

// Execute 实现 Node 接口
func (n *TCListDirNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	path := params["path"]
	if path == "" {
		return "", fmt.Errorf("path parameter is required")
	}

	safePath, err := validateReadPath(path)
	if err != nil {
		return "", fmt.Errorf("path validation failed: %w", err)
	}

	info, err := os.Stat(safePath)
	if err != nil {
		return "", fmt.Errorf("failed to stat path: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", path)
	}

	recursive := getParam(params, "recursive", "false") == "true"
	maxEntries := tcDefaultListEntries
	if v := params["max_entries"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxEntries = n
		}
	}
	if maxEntries > tcHardMaxListEntries {
		maxEntries = tcHardMaxListEntries
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}

	var entries []string
	walkErr := filepath.WalkDir(safePath, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		// 跳过符号链接
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel, rerr := filepath.Rel(safePath, p)
		if rerr != nil {
			return nil
		}
		// 跳过根目录本身
		if rel == "." {
			return nil
		}
		// 递归深度限制
		depth := tcRelDepth(rel)
		if d.IsDir() {
			if depth > tcMaxListDirDepth {
				return filepath.SkipDir
			}
			if !recursive && depth > 1 {
				return filepath.SkipDir
			}
		} else {
			if depth > tcMaxListDirDepth {
				return nil
			}
			if !recursive && depth > 1 {
				return nil
			}
		}
		// 条目数限制
		if len(entries) >= maxEntries {
			return filepath.SkipAll
		}
		entries = append(entries, tcFormatDirEntry(rel, d))
		return nil
	})
	if walkErr != nil {
		return "", fmt.Errorf("walk failed: %w", walkErr)
	}

	logger.Info("list_dir entries", "path", path, "count", len(entries))
	return strings.Join(entries, "\n"), nil
}

// tcFormatDirEntry 格式化目录条目：relpath\ttype\tsize
// type 为 "dir" 或 "file"；size 对目录为 "-"，对文件为字节数
func tcFormatDirEntry(rel string, d os.DirEntry) string {
	rel = tcSanitizePath(filepath.ToSlash(rel))
	if d.IsDir() {
		return rel + "\tdir\t-"
	}
	info, err := d.Info()
	if err != nil {
		return rel + "\tfile\t-"
	}
	return fmt.Sprintf("%s\tfile\t%d", rel, info.Size())
}

// ----------------------------------------------------------------------------
// apply_patch 工具：应用 unified diff 补丁（借鉴 Codex apply_patch）
// ----------------------------------------------------------------------------

// TCApplyPatchNode 应用 unified diff 补丁节点
//
// 这是高风险操作，必须满足：
//   - 每个目标文件用 validateWritePath 校验
//   - 拒绝绝对路径和 .. 路径
//   - 补丁大小限制 1MB
//   - 仅支持 unified diff 格式
//   - 最多修改 50 个文件
//   - 操作前可选备份（默认写入 .bak）
//   - 原子语义：先验证全部补丁格式与上下文匹配，再应用；任一步失败则不做任何修改
type TCApplyPatchNode struct{}

func init() {
	Register(&TCApplyPatchNode{})
}

// Name 返回节点名
func (n *TCApplyPatchNode) Name() string { return "apply_patch" }

// Description 返回节点描述
func (n *TCApplyPatchNode) Description() string {
	return "Apply a unified diff patch (compat: Codex apply_patch)"
}

// Schema 返回节点 schema
func (n *TCApplyPatchNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "apply_patch",
		Description: "解析并应用 unified diff 格式补丁到文件。原子语义：全部校验通过后才应用。兼容 Codex 的 apply_patch 工具。",
		Input:       "string - unified diff 补丁内容（也可通过 patch 参数传入）",
		Output:      "string - 应用结果摘要",
		Params: []ParamSchema{
			{Name: "patch", Type: "string", Description: "unified diff 补丁内容（与 input 二选一）", Required: false},
			{Name: "backup", Type: "string", Description: "是否在应用前备份原文件到 .bak：true/false（默认 true）", Required: false, Default: "true"},
		},
	}
}

// Execute 实现 Node 接口
func (n *TCApplyPatchNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	// 补丁内容来源：patch 参数优先，否则用 input
	patchContent := params["patch"]
	if patchContent == "" {
		patchContent = input
	}
	if patchContent == "" {
		return "", fmt.Errorf("patch content is required (pass via input or patch param)")
	}
	if len(patchContent) > tcMaxPatchSize {
		return "", fmt.Errorf("patch too large (max %d bytes)", tcMaxPatchSize)
	}

	backup := getParam(params, "backup", "true") == "true"

	// 第一阶段：解析与验证（不修改任何文件）
	filePatches, err := tcParsePatch(patchContent)
	if err != nil {
		return "", fmt.Errorf("patch parse failed: %w", err)
	}
	if len(filePatches) == 0 {
		return "", fmt.Errorf("no file patches found in input")
	}
	if len(filePatches) > tcMaxPatchFiles {
		return "", fmt.Errorf("too many files in patch (max %d, got %d)", tcMaxPatchFiles, len(filePatches))
	}

	// 校验每个目标路径并准备新内容
	pending := make([]tcPending, 0, len(filePatches))
	for _, fp := range filePatches {
		safePath, err := validateWritePath(fp.targetPath)
		if err != nil {
			return "", fmt.Errorf("target path validation failed for %q: %w", fp.targetPath, err)
		}
		// 读取原文件内容（不存在则视为新建）
		var origData []byte
		isNew := false
		if _, statErr := os.Stat(safePath); os.IsNotExist(statErr) {
			isNew = true
			origData = nil
		} else if statErr != nil {
			return "", fmt.Errorf("failed to stat target %q: %w", fp.targetPath, statErr)
		} else {
			data, readErr := os.ReadFile(safePath)
			if readErr != nil {
				return "", fmt.Errorf("failed to read target %q: %w", fp.targetPath, readErr)
			}
			origData = data
		}
		// 应用 hunks 到内存副本，校验上下文匹配
		origLines := tcSplitLines(string(origData))
		newLines, applyErr := tcApplyHunks(origLines, fp.hunks)
		if applyErr != nil {
			return "", fmt.Errorf("hunk application failed for %q: %w", fp.targetPath, applyErr)
		}
		newContent := strings.Join(newLines, "\n")
		// 保持空文件语义：若 origData 为空且 newLines 也为空，结果为空串
		if len(origData) == 0 && len(newLines) == 0 {
			newContent = ""
		}
		pending = append(pending, tcPending{
			safePath:   safePath,
			origPath:   fp.targetPath,
			newContent: newContent,
			isNew:      isNew,
		})
	}

	// 第二阶段：暂存（将每个新内容写入临时文件，不触碰最终文件）
	for i := range pending {
		p := &pending[i]
		// 创建必要的父目录（新文件可能位于尚不存在的目录）
		dir := filepath.Dir(p.safePath)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o700); err != nil {
				tcCleanupTemps(pending, i)
				return "", fmt.Errorf("mkdir failed for %q: %w (no files modified)", p.origPath, err)
			}
		}
		p.tmpPath = p.safePath + ".tmp.applypatch"
		// 拒绝已存在的符号链接目标（防 symlink 重定向攻击：攻击者预先创建
		// file.tmp.applypatch -> /etc/critical 的符号链接，使新内容被写入目标之外）
		if li, lerr := os.Lstat(p.tmpPath); lerr == nil && li.Mode()&os.ModeSymlink != 0 {
			tcCleanupTemps(pending, i)
			return "", fmt.Errorf("temp path is a symlink, refusing to write: %q (no files modified)", p.origPath)
		}
		if err := os.WriteFile(p.tmpPath, []byte(p.newContent), 0o600); err != nil {
			tcCleanupTemps(pending, i)
			_ = os.Remove(p.tmpPath) // best-effort cleanup
			return "", fmt.Errorf("stage write failed for %q: %w (no files modified)", p.origPath, err)
		}
	}

	// 第三阶段：提交（备份 + 原子 rename 临时文件 → 最终文件）
	// 注意：rename(2) 在同一文件系统内是原子的，不会留下半写入状态。
	var applied []string
	for i := range pending {
		p := &pending[i]
		if backup && !p.isNew {
			bakPath := p.safePath + ".bak"
			if err := tcBackupFile(p.safePath, bakPath); err != nil {
				tcCleanupTemps(pending, i+1)
				return "", fmt.Errorf("backup failed for %q: %w (earlier files already committed)", p.origPath, err)
			}
		}
		if err := os.Rename(p.tmpPath, p.safePath); err != nil {
			tcCleanupTemps(pending, i+1)
			return "", fmt.Errorf("commit (rename) failed for %q: %w (earlier files already committed)", p.origPath, err)
		}
		applied = append(applied, p.origPath)
	}

	logger.Info("apply_patch succeeded", "files", len(applied), "backup", backup)
	summary := fmt.Sprintf("patch applied to %d file(s)", len(applied))
	for _, p := range applied {
		summary += "\n  - " + p
	}
	return summary, nil
}

// tcDiffLine 描述 diff 中的一行
type tcDiffLine struct {
	kind    byte // ' ' context, '-' deletion, '+' addition
	content string
}

// tcHunk 描述一个 diff hunk
type tcHunk struct {
	oldStart int
	oldCount int
	newStart int
	newCount int
	lines    []tcDiffLine
}

// tcFilePatch 描述单个文件的补丁
type tcFilePatch struct {
	targetPath string // 校验后用于显示的相对路径（b/ 已剥离）
	isNewFile  bool
	hunks      []tcHunk
}

// tcPending 描述一个已校验、待提交的文件补丁（含临时文件路径）
type tcPending struct {
	safePath   string
	origPath   string
	newContent string
	isNew      bool
	tmpPath    string
}

// tcParsePatch 解析 unified diff 补丁，返回按文件分组的补丁列表
// 解析失败时返回错误，不做任何文件修改
func tcParsePatch(content string) ([]tcFilePatch, error) {
	// 统一换行符并去除末尾单个换行符（split 会产生多余空串，
	// 该空串会被误当作 hunk 上下文行，导致行数校验失败）
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.TrimSuffix(content, "\n")
	lines := strings.Split(content, "\n")
	var patches []tcFilePatch
	var current *tcFilePatch
	var currentHunk *tcHunk
	// 跟踪 --- 行是否为 /dev/null（--- 出现在 +++ 之前，需暂存）
	pendingSrcIsNew := false

	finishHunk := func() error {
		if currentHunk == nil {
			return nil
		}
		// 校验 hunk 行数与声明的 count 是否一致
		oldN, newN := 0, 0
		for _, l := range currentHunk.lines {
			switch l.kind {
			case ' ':
				oldN++
				newN++
			case '-':
				oldN++
			case '+':
				newN++
			}
		}
		if currentHunk.oldCount != 0 && oldN != currentHunk.oldCount {
			return fmt.Errorf("hunk old line count mismatch: declared %d, actual %d", currentHunk.oldCount, oldN)
		}
		if currentHunk.newCount != 0 && newN != currentHunk.newCount {
			return fmt.Errorf("hunk new line count mismatch: declared %d, actual %d", currentHunk.newCount, newN)
		}
		current.hunks = append(current.hunks, *currentHunk)
		currentHunk = nil
		return nil
	}
	finishFile := func() error {
		if current == nil {
			return nil
		}
		if err := finishHunk(); err != nil {
			return err
		}
		if len(current.hunks) == 0 {
			return fmt.Errorf("file patch has no hunks: %s", current.targetPath)
		}
		patches = append(patches, *current)
		current = nil
		return nil
	}

	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "+++ "):
			// 开始新文件块前，先结束上一个
			if err := finishFile(); err != nil {
				return nil, err
			}
			target, err := tcParseDiffTarget(line)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", i+1, err)
			}
			// isNewFile 由前导的 --- /dev/null 行决定（标准新文件约定）
			current = &tcFilePatch{targetPath: target, isNewFile: pendingSrcIsNew}
			pendingSrcIsNew = false
		case strings.HasPrefix(line, "--- "):
			// 源行：检测 /dev/null 标记新建文件，暂存到 pendingSrcIsNew
			// （--- 出现在 +++ 之前，需在 +++ 时再应用到 current）
			pendingSrcIsNew = strings.Contains(line, "/dev/null")
		case strings.HasPrefix(line, "@@ "):
			if current == nil {
				return nil, fmt.Errorf("line %d: hunk header outside file patch", i+1)
			}
			if err := finishHunk(); err != nil {
				return nil, fmt.Errorf("line %d: %w", i+1, err)
			}
			m := tcHunkHeaderRe.FindStringSubmatch(line)
			if m == nil {
				return nil, fmt.Errorf("line %d: invalid hunk header: %q", i+1, line)
			}
			oldStart, _ := strconv.Atoi(m[1])
			oldCount := 1
			if m[2] != "" {
				oldCount, _ = strconv.Atoi(m[2])
			}
			newStart, _ := strconv.Atoi(m[3])
			newCount := 1
			if m[4] != "" {
				newCount, _ = strconv.Atoi(m[4])
			}
			currentHunk = &tcHunk{
				oldStart: oldStart,
				oldCount: oldCount,
				newStart: newStart,
				newCount: newCount,
			}
		case strings.HasPrefix(line, "\\ "):
			// "\ No newline at end of file" 标记，忽略
			continue
		case currentHunk != nil && (strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") || strings.HasPrefix(line, " ") || line == ""):
			// hunk 行。空行视为上下文行（diff 中空行的前缀是空格，但某些工具会丢失）
			var kind byte
			content := ""
			if line == "" {
				kind = ' '
				content = ""
			} else {
				kind = line[0]
				content = line[1:]
			}
			if kind != ' ' && kind != '-' && kind != '+' {
				return nil, fmt.Errorf("line %d: unexpected diff line prefix %q", i+1, kind)
			}
			currentHunk.lines = append(currentHunk.lines, tcDiffLine{kind: kind, content: content})
		default:
			// 跳过无法识别的行（可能是 diff --git 等元信息行）
			continue
		}
	}
	if err := finishFile(); err != nil {
		return nil, err
	}
	return patches, nil
}

// tcParseDiffTarget 解析 +++ b/path 行，返回目标路径
// 拒绝 /dev/null 与绝对路径
func tcParseDiffTarget(line string) (target string, err error) {
	rest := strings.TrimPrefix(line, "+++ ")
	rest = strings.TrimSpace(rest)
	// 去除行尾时间戳（如 "b/path\t2024-01-01 12:00:00"）
	if idx := strings.IndexByte(rest, '\t'); idx >= 0 {
		rest = rest[:idx]
	}
	// 去除可能的引号包裹
	if len(rest) >= 2 && rest[0] == '"' && rest[len(rest)-1] == '"' {
		rest = rest[1 : len(rest)-1]
	}
	if rest == "/dev/null" {
		return "", fmt.Errorf("file deletion (/dev/null in +++) is not supported")
	}
	// 拒绝绝对路径
	if filepath.IsAbs(rest) {
		return "", fmt.Errorf("absolute target path not allowed: %s", rest)
	}
	// 拒绝含 .. 的路径
	if strings.Contains(rest, "..") {
		return "", fmt.Errorf("target path must not contain '..': %s", rest)
	}
	// 剥离 b/ 前缀（unified diff 约定）
	target = strings.TrimPrefix(rest, "b/")
	if target == "" {
		return "", fmt.Errorf("empty target path in +++ header")
	}
	return target, nil
}

// tcSplitLines 将内容按 \n 切分为行（保留尾部空串以反映结尾换行符语义）
// 例如 "a\nb\n" → ["a","b",""]，"a\nb" → ["a","b"]
func tcSplitLines(content string) []string {
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}

// tcApplyHunks 将一组 hunk 应用到原文件的行列表，返回新文件的行列表
// 任一上下文不匹配则返回错误（atomic：调用方据此放弃修改）
func tcApplyHunks(origLines []string, hunks []tcHunk) ([]string, error) {
	var result []string
	origIdx := 0
	for _, hunk := range hunks {
		// oldStart 是 1-indexed；转为 0-indexed。oldStart==0 表示新文件
		targetIdx := hunk.oldStart - 1
		if targetIdx < 0 {
			targetIdx = 0
		}
		// 复制 targetIdx 之前未触及的原文件行
		for origIdx < targetIdx && origIdx < len(origLines) {
			result = append(result, origLines[origIdx])
			origIdx++
		}
		// 应用 hunk 行
		for _, dl := range hunk.lines {
			switch dl.kind {
			case ' ': // 上下文行：必须匹配
				if origIdx >= len(origLines) {
					return nil, fmt.Errorf("context line beyond end of file at hunk @@ -%d", hunk.oldStart)
				}
				if origLines[origIdx] != dl.content {
					return nil, fmt.Errorf("context mismatch at line %d: expected %q, got %q",
						origIdx+1, dl.content, origLines[origIdx])
				}
				result = append(result, origLines[origIdx])
				origIdx++
			case '-': // 删除行：必须匹配
				if origIdx >= len(origLines) {
					return nil, fmt.Errorf("deletion line beyond end of file at hunk @@ -%d", hunk.oldStart)
				}
				if origLines[origIdx] != dl.content {
					return nil, fmt.Errorf("deletion mismatch at line %d: expected %q, got %q",
						origIdx+1, dl.content, origLines[origIdx])
				}
				origIdx++
			case '+': // 新增行：直接追加到结果
				result = append(result, dl.content)
			}
		}
	}
	// 复制剩余未触及的原文件行
	for origIdx < len(origLines) {
		result = append(result, origLines[origIdx])
		origIdx++
	}
	return result, nil
}

// tcCleanupTemps 删除 pending 切片中从 from 开始的所有临时文件（用于失败回滚）
func tcCleanupTemps(pending []tcPending, from int) {
	for i := from; i < len(pending); i++ {
		if pending[i].tmpPath != "" {
			_ = os.Remove(pending[i].tmpPath) // best-effort cleanup
		}
	}
}

// tcBackupFile 将 src 复制到 dst（用于应用补丁前备份）
func tcBackupFile(src, dst string) error {
	// 拒绝符号链接目标（防 symlink 重定向攻击：避免原文件内容被写入
	// 攻击者预先以符号链接指向的敏感路径）
	if li, err := os.Lstat(dst); err == nil && li.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("backup target is a symlink: %s", dst)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}
