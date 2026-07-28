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
