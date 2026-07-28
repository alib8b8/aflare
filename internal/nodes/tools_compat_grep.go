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
	data, err := os.ReadFile(absPath) // #nosec G304 -- path validated by safeJoinPath
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
