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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alib8b8/llm-box/internal/logger"
)

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
			data, readErr := os.ReadFile(safePath) // #nosec G304 -- path validated by safeJoinPath
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
	data, err := os.ReadFile(src) // #nosec G304 -- path validated by safeJoinPath
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}
