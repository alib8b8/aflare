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
)

const maxFileReadSize = 10 * 1024 * 1024 // 10MB max file read size

type FileReadNode struct{}

func init() {
	Register(&FileReadNode{})
}

func (n *FileReadNode) Name() string {
	return "file_read"
}

func (n *FileReadNode) Description() string {
	return "Read content from a file"
}

func (n *FileReadNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "file_read",
		Description: "Read content from a file. Automatically redacts secrets (API keys, tokens, .env files) by default for privacy — set redact=false to disable.",
		Input:       "string - not used",
		Output:      "string - file content (with secrets redacted by default)",
		Params: []ParamSchema{
			{Name: "path", Type: "string", Description: "File path to read from", Required: true},
			{Name: "redact", Type: "string", Description: "Redact secrets in output: true (default) / false. When true, .env and credential files are fully masked; other files have known secret patterns masked.", Required: false, Default: "true"},
		},
	}
}

func (n *FileReadNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	path, ok := params["path"]
	if !ok || path == "" {
		return "", fmt.Errorf("path parameter is required")
	}

	safePath, err := validateReadPath(path)
	if err != nil {
		return "", fmt.Errorf("path validation failed: %w", err)
	}

	fi, err := os.Stat(safePath)
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}
	if fi.Size() > maxFileReadSize {
		return "", fmt.Errorf("file too large (max %d bytes)", maxFileReadSize)
	}

	data, err := os.ReadFile(safePath) // #nosec G304 -- path validated by validateReadPath
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	content := string(data)

	// 隐私优先：默认对读取内容进行密钥脱敏（借鉴 Grok Build 隐私丑闻教训）
	redact := getParam(params, "redact", "true")
	if redact != "false" {
		// 敏感文件（.env / 私钥 / credentials）整文件脱敏
		wholeFile := IsSensitiveFile(filepath.Base(safePath))
		masked, hits := RedactSecrets(content, wholeFile)
		if hits > 0 {
			// 记录脱敏行为（不泄露文件内容，仅记录命中数）
			return masked + fmt.Sprintf("\n\n[security: %d secret(s) redacted]", hits), nil
		}
	}

	return content, nil
}
