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
	"strings"

	"github.com/alib8b8/llm-box/internal/logger"
)

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
